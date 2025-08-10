package httphandler

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shorya-1012/gnpm/internal/constants"
	"github.com/shorya-1012/gnpm/internal/models"
)

type HttpHandler struct {
	httpClient *http.Client
}

func NewHttpHandler() *HttpHandler {
	handler := HttpHandler{
		httpClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  false,
				ForceAttemptHTTP2:   true,
			},
		},
	}
	return &handler
}

func createRequest(requestUrl string) *http.Request {
	request, err := http.NewRequest("GET", requestUrl, nil)
	if err != nil {
		fmt.Println("Unable to create request")
		fmt.Println(err)
	}

	// I don't actually know what this does
	//  but somehow this makes the request faster by reducing response body size
	request.Header.Set("Accept", "application/vnd.npm.install-v1+json; q=1.0, application/json; q=0.8, */*")
	return request
}

// should be prioritised lesser response size
func (h *HttpHandler) FetchMetaDataWithVersion(packageName string, fullVersion string) (models.PackageVersionMetadata, error) {

	requestUrl := fmt.Sprintf("%s/%s/%s", constants.RegistryBaseURL, packageName, fullVersion)
	request := createRequest(requestUrl)

	response, err := h.httpClient.Do(request)
	if err != nil {
		return models.PackageVersionMetadata{}, fmt.Errorf("Unable to get response from registry for %s %s: %w", packageName, fullVersion, err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return models.PackageVersionMetadata{}, fmt.Errorf("Registry returned status %d for %s %s", response.StatusCode, packageName, fullVersion)
	}

	var packageMetaData models.PackageVersionMetadata

	if err := json.NewDecoder(response.Body).Decode(&packageMetaData); err != nil {
		fmt.Println("Error Decoding Json : ")
		return models.PackageVersionMetadata{}, fmt.Errorf("Error decoding JSON for %s %s: %w", packageName, fullVersion, err)
	}

	return packageMetaData, nil
}

// should be avoided as slower
func (h *HttpHandler) FetchFullMetaData(packageName string) (models.PackageMetadata, error) {
	requestUrl := fmt.Sprintf("%s/%s", constants.RegistryBaseURL, packageName)
	request := createRequest(requestUrl)

	response, err := h.httpClient.Do(request)
	if err != nil {
		return models.PackageMetadata{}, fmt.Errorf("Unable to get response from registry for %s: %w", packageName, err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return models.PackageMetadata{}, fmt.Errorf("Registry returned status %d for %s ", response.StatusCode, packageName)
	}

	var packageMetaData models.PackageMetadata

	if err := json.NewDecoder(response.Body).Decode(&packageMetaData); err != nil {
		return models.PackageMetadata{}, fmt.Errorf("Error decoding JSON for %s : %w", packageName, err)
	}

	return packageMetaData, nil
}

func (h *HttpHandler) DownloadAndInstallTarBall(url string, destDir string) error {
	response, err := h.httpClient.Get(url)

	if err != nil {
		fmt.Println("Unable to get Tarball")
		fmt.Println(err)
	}
	defer response.Body.Close()

	gzReader, err := gzip.NewReader(response.Body)
	if err != nil {
		fmt.Println("Unable to create gzReader")
		fmt.Println(err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err)
		}

		relPath := strings.TrimPrefix(header.Name, "package/")
		targetPath := filepath.Join(destDir, relPath)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("mkdir failed: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("mkdir parent failed: %w", err)
			}
			outFile, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("create file failed: %w", err)
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("write file failed: %w", err)
			}
			outFile.Close()
		default:
			//ignore
		}

	}
	return nil
}
