package lib

import (
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strings"
)

type tfVersionList struct {
	tflist []string
}

func getVersionsFromBody(body string, preRelease bool, tfVersionList *tfVersionList) {
	var semver string
	if preRelease {
		// Getting versions from body; should return match /X.X.X-@/ where X is a number,@ is a word character between a-z or A-Z
		semver = `\/?(\d+\.\d+\.\d+)(-[a-zA-z]+\d*)?/?"`
	} else if !preRelease {
		// Getting versions from body; should return match /X.X.X/ where X is a number
		// without the ending '"' pre-release folders would be tried and break.
		semver = `\/?(\d+\.\d+\.\d+)\/?"`
	}
	r, _ := regexp.Compile(semver)

	matches := r.FindAllString(body, -1)
	if matches == nil {
		return
	}
	for _, match := range matches {
		trimstr := strings.Trim(match, "/\"") // remove '/' or '"' from /X.X.X/" or /X.X.X"
		tfVersionList.tflist = append(tfVersionList.tflist, trimstr)
	}
}

// getTFList :  Get the list of available versions given the mirror URL
func getTFList(mirrorURL string, preRelease bool) ([]string, error) {
	logger.Debug("Getting list of versions")
	result, err := getTFURLBody(mirrorURL)
	if err != nil {
		return nil, err
	}

	var tfVersionList tfVersionList
	getVersionsFromBody(result, preRelease, &tfVersionList)

	if len(tfVersionList.tflist) == 0 {
		logger.Errorf("Cannot get version list from mirror: %s", mirrorURL)
	}
	return tfVersionList.tflist, nil
}

// getTFLatest :  Get the latest terraform version given the hashicorp url
func getTFLatest(mirrorURL string) (string, error) {
	result, err := getTFURLBody(mirrorURL)
	if err != nil {
		return "", err
	}
	// Getting versions from body; should return match /X.X.X/ where X is a number
	semver := `\/?(\d+\.\d+\.\d+)\/?"`
	r, _ := regexp.Compile(semver)
	bodyLines := strings.Split(result, "\n")
	for i := range result {
		if r.MatchString(bodyLines[i]) {
			str := r.FindString(bodyLines[i])
			trimstr := strings.Trim(str, "/\"") // remove '/' or '"' from /X.X.X/" or /X.X.X"
			return trimstr, nil
		}
	}
	return "", nil
}

// getTFLatestImplicit :  Get the latest implicit terraform version given the hashicorp url
func getTFLatestImplicit(mirrorURL string, preRelease bool, version string) (string, error) {
	if preRelease {
		// TODO: use getTFList() instead of getTFURLBody
		body, error := getTFURLBody(mirrorURL)
		if error != nil {
			return "", error
		}
		// Getting versions from body; should return match /X.X.X-@/ where X is a number,@ is a word character between a-z or A-Z
		semver := fmt.Sprintf(`\/?(%s{1}\.\d+\-[a-zA-z]+\d*)\/?"`, version)
		r, err := regexp.Compile(semver)
		if err != nil {
			return "", err
		}
		versions := strings.Split(body, "\n")
		for i := range versions {
			if r.MatchString(versions[i]) {
				str := r.FindString(versions[i])
				trimstr := strings.Trim(str, "/\"") // remove '/' or '"' from /X.X.X/" or /X.X.X"
				return trimstr, nil
			}
		}
	} else if !preRelease {
		listAll := false
		tflist, _ := getTFList(mirrorURL, listAll) // get list of versions
		version = fmt.Sprintf("~> %v", version)
		semv, err := SemVerParser(&version, tflist)
		if err != nil {
			return "", err
		}
		return semv, nil
	}
	return "", nil
}

// getTFURLBody : Get list of terraform versions from hashicorp releases
func getTFURLBody(mirrorURL string) (string, error) {
	hasSlash := strings.HasSuffix(mirrorURL, "/")
	if !hasSlash {
		// if it does not have slash - append slash
		mirrorURL = fmt.Sprintf("%s/", mirrorURL)
	}
	resp, errURL := http.Get(mirrorURL)
	if errURL != nil {
		logger.Fatalf("Error getting url: %v", errURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		logger.Fatalf("Error retrieving contents from url: %s", mirrorURL)
	}

	body, errBody := io.ReadAll(resp.Body)
	if errBody != nil {
		logger.Fatalf("Error reading body: %v", errBody)
	}

	bodyString := string(body)

	return bodyString, nil
}

// versionExist : check if requested version exist
func versionExist(val interface{}, array interface{}) (exists bool) {
	exists = false
	switch reflect.TypeOf(array).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(array)

		for i := 0; i < s.Len(); i++ {
			if reflect.DeepEqual(val, s.Index(i).Interface()) {
				exists = true
				return exists
			}
		}
	default:
		panic("unhandled default case")
	}
	return exists
}

// removeDuplicateVersions : remove duplicate version
func removeDuplicateVersions(elements []string) []string {
	// Use map to record duplicates as we find them.
	encountered := map[string]bool{}
	result := []string{}

	for _, val := range elements {
		versionOnly := strings.Trim(val, " *recent")
		if encountered[versionOnly] {
			// Do not add duplicate.
		} else {
			// Record this element as an encountered element.
			encountered[versionOnly] = true
			// Append to result slice.
			result = append(result, val)
		}
	}
	// Return the new slice.
	return result
}

// validVersionFormat : returns valid version format
/* For example: 0.1.2 = valid
// For example: 0.1.2-beta1 = valid
// For example: 0.1.2-alpha = valid
// For example: a.1.2 = invalid
// For example: 0.1. 2 = invalid
*/
func validVersionFormat(version string) bool {
	// Getting versions from body; should return match /X.X.X-@/ where X is a number,@ is a word character between a-z or A-Z
	// Follow https://semver.org/spec/v1.0.0-beta.html
	// Check regular expression at https://rubular.com/r/ju3PxbaSBALpJB
	semverRegex := regexp.MustCompile(`^(\d+\.\d+\.\d+)(-[a-zA-z]+\d*)?$`)
	return semverRegex.MatchString(version)
}

// ValidMinorVersionFormat : returns valid MINOR version format
/* For example: 0.1 = valid
// For example: a.1.2 = invalid
// For example: 0.1.2 = invalid
*/
func validMinorVersionFormat(version string) bool {
	// Getting versions from body; should return match /X.X./ where X is a number
	semverRegex := regexp.MustCompile(`^(\d+\.\d+)$`)

	return semverRegex.MatchString(version)
}

// ShowLatestVersion show install latest stable tf version
func ShowLatestVersion(mirrorURL string) {
	tfversion, _ := getTFLatest(mirrorURL)
	fmt.Printf("%s\n", tfversion)
}

// ShowLatestImplicitVersion show latest - argument (version) must be provided
func ShowLatestImplicitVersion(requestedVersion, mirrorURL string, preRelease bool) {
	if validMinorVersionFormat(requestedVersion) {
		tfversion, _ := getTFLatestImplicit(mirrorURL, preRelease, requestedVersion)
		if len(tfversion) > 0 {
			fmt.Printf("%s\n", tfversion)
		} else {
			logger.Fatalf("Requested version does not exist: %q.\n\tTry `tfswitch -l` to see all available versions", requestedVersion)
		}
	} else {
		PrintInvalidMinorTFVersion()
	}
}
