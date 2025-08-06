package httphandler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/shorya-1012/gnpm/internal/constants"
	"github.com/shorya-1012/gnpm/internal/models"
)

// should be prioritised
func FetchMetaDataWithVersion(packageName string, fullVersion string) models.PackageVersionMetadata {
	response, err := http.Get(fmt.Sprintf("%s/%s/%s", constants.RegistryBaseURL, packageName, fullVersion))
	if err != nil {
		fmt.Println("Unable to get metadata from registry")
	}
	defer response.Body.Close()
	var packageMetaData models.PackageVersionMetadata

	if err := json.NewDecoder(response.Body).Decode(&packageMetaData); err != nil {
		fmt.Println("Error Decoding Json : ")
		fmt.Println(packageName, ": ", fullVersion)
		fmt.Println(err)
	}

	return packageMetaData
}

// should be avoided as slower
func FetchFullMetaData(packageName string) models.PackageMetadata {
	response, err := http.Get(fmt.Sprintf("%s/%s", constants.RegistryBaseURL, packageName))
	if err != nil {
		fmt.Println("Unable to get metadata from registry")
	}
	defer response.Body.Close()
	var packageMetaData models.PackageMetadata

	if err := json.NewDecoder(response.Body).Decode(&packageMetaData); err != nil {
		fmt.Println("Error Decoding Json")
		fmt.Println(packageName)
		fmt.Println(err)
	}

	return packageMetaData
}
