package httphandler

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/pgzip"
	"github.com/shorya-1012/gnpm/internal/constants"
	"github.com/shorya-1012/gnpm/internal/models"
)

type HttpHandler struct {
	httpClient    *http.Client
	createDirs    map[string]struct{}
	dirCacheMutex sync.RWMutex
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
		createDirs: make(map[string]struct{}),
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

func (h *HttpHandler) DownloadTarBall(url string) (io.ReadCloser, error) {
	resp, err := h.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (h *HttpHandler) InstallTarBall(response io.Reader, destDir string) error {
	gzReader, err := pgzip.NewReaderN(response, 1<<20, 8)
	if err != nil {
		return fmt.Errorf("Failed to create gzip reader : %w", err)
	}

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("Unable to read tar : %w", err)
		}

		// remove "pacakage/" prefix
		relPath := strings.TrimPrefix(header.Name, "package/")
		targetPath := filepath.Join(destDir, relPath)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := h.ensureDir(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create dir: %w", err)
			}
		case tar.TypeReg:
			// Ensure parent dir exists
			if err := h.ensureDir(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent dir: %w", err)
			}

			outFile, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			// Stream file contents directly
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write file: %w", err)
			}
			outFile.Close()
		}
	}

	return nil
}

func (h *HttpHandler) ensureDir(path string, mode os.FileMode) error {
	h.dirCacheMutex.Lock()
	if _, exists := h.createDirs[path]; exists {
		h.dirCacheMutex.Unlock()
		return nil
	}
	h.createDirs[path] = struct{}{}
	h.dirCacheMutex.Unlock()
	return os.MkdirAll(path, mode)
}
