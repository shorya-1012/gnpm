package installer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/Masterminds/semver"
	"github.com/shorya-1012/gnpm/internal/http_handler"
	"github.com/shorya-1012/gnpm/internal/models"
	"github.com/shorya-1012/gnpm/internal/parser"
)

type Installer struct {
	Dependencies      models.DependencyMap
	ResolvedMap       map[string][]*semver.Version
	metaDataCache     map[string]models.PackageVersionMetadata
	fullMetaDataCache map[string]models.PackageMetadata
	downloadCache     map[string]struct{}

	depsMutex         sync.RWMutex
	resolvedMutex     sync.RWMutex
	versionCacheMutex sync.RWMutex
	cacheMutex        sync.RWMutex
	downloadMutex     sync.RWMutex

	httpHandler *httphandler.HttpHandler

	resolveChan  chan InstallTask
	downloadChan chan DownloadTask
	extractChan  chan ExtractTask
	resolveWg    sync.WaitGroup
	downloadWg   sync.WaitGroup
	extractWg    sync.WaitGroup

	sem chan struct{}
}

type InstallTask struct {
	Name    string
	Version string
	Parent  *string
}

type DownloadTask struct {
	name        string
	tarball     string
	installPath string
}

type ExtractTask struct {
	Name     string
	Reader   io.ReadCloser
	DestPath string
}

func NewInstaller() *Installer {
	installer := Installer{
		Dependencies:      make(models.DependencyMap),
		ResolvedMap:       make(map[string][]*semver.Version),
		metaDataCache:     make(map[string]models.PackageVersionMetadata),
		fullMetaDataCache: make(map[string]models.PackageMetadata),
		downloadCache:     make(map[string]struct{}),
		resolveChan:       make(chan InstallTask, 64),
		downloadChan:      make(chan DownloadTask, 32),
		extractChan:       make(chan ExtractTask, 32),
		sem:               make(chan struct{}, 8),
		httpHandler:       httphandler.NewHttpHandler(),
	}
	return &installer
}

// entry point for the installer
func (i *Installer) HandleInstall(packageName string) {
	baseWorkerCount := runtime.NumCPU()

	for w := 0; w < baseWorkerCount*2; w++ {
		go i.resolveWorker()
	}

	for w := 0; w < baseWorkerCount; w++ {
		go i.workerDownloader()
	}

	for w := 0; w < baseWorkerCount; w++ {
		go i.workerExtractor()
	}

	packageName, packageVersion := parser.ParseVersion(packageName)
	i.enqueueResolveTask(packageName, packageVersion, nil)

	i.resolveWg.Wait()
	close(i.resolveChan)

	i.downloadWg.Wait()
	close(i.downloadChan)

	i.extractWg.Wait()
	close(i.extractChan)
}

func (i *Installer) installPackage(packageName string, packageVersion string, parent *string) {
	fullVersion, _ := semver.NewVersion(packageVersion)
	var pkgMetadata models.PackageVersionMetadata

	/*
	 semver.NewVersion only return a value if the given stirng is a complete sem version,
	 it thows an error if the provided string is a constraint in which case we have to resolve it
	*/

	if fullVersion == nil {
		pkgMetadata = i.handleSemVerInstall(&packageName, &packageVersion)
		version, err := semver.NewVersion(pkgMetadata.Version)
		// for debugging for now
		if err != nil {
			fmt.Println("Unable to parse version : ", packageName, " ", pkgMetadata.Version)
			fmt.Println(err)
		}
		fullVersion = version
	} else {
		versionString := fmt.Sprintf("%s@%s", packageName, fullVersion.String())
		if i.isResolved(versionString) {
			if parent != nil {
				i.appendVersion(parent, versionString)
			}
			return
		}

		pkgMetadata = i.fetchMetaDataWithCache(packageName, fullVersion.String())
	}

	pkgInfoStringified := fmt.Sprintf("%s@%s", packageName, fullVersion.String())

	if parent != nil {
		i.appendVersion(parent, pkgInfoStringified)
	}

	// resolve dependencies recursively
	for depName, depVersion := range pkgMetadata.Dependencies {
		i.enqueueResolveTask(depName, depVersion, &packageName)
	}

	i.resolvedMutex.Lock()
	resolvedInfo, found := i.ResolvedMap[packageName]

	if !found {
		resolvedInfo = make([]*semver.Version, 0)
	}

	resolvedInfo = append(resolvedInfo, fullVersion)
	i.ResolvedMap[packageName] = resolvedInfo
	i.resolvedMutex.Unlock()

	key := pkgInfoStringified
	i.downloadMutex.Lock()
	_, alreadyDownloading := i.downloadCache[key]
	if !alreadyDownloading {
		i.downloadCache[key] = struct{}{}
	}

	i.downloadMutex.Unlock()

	if !alreadyDownloading {
		i.downloadCache[key] = struct{}{}
		i.downloadWg.Add(1)
		i.downloadChan <- DownloadTask{
			name:        packageName,
			tarball:     pkgMetadata.Dist.Tarball,
			installPath: filepath.Join("node_modules", packageName),
		}
	}

}

func (i *Installer) isResolved(packageInfo string) bool {
	i.depsMutex.Lock()
	defer i.depsMutex.Unlock()

	_, found := i.Dependencies[packageInfo]

	if found {
		return true
	}

	i.Dependencies[packageInfo] = models.PackageInfo{
		Dependencies: []string{},
	}
	return false
}

// append version
func (i *Installer) appendVersion(parent *string, packageVersion string) {
	i.depsMutex.Lock()
	defer i.depsMutex.Unlock()

	parentInfo, found := i.Dependencies[*parent]

	if !found {
		parentInfo = models.PackageInfo{
			Dependencies: []string{},
		}
	}

	if slices.Contains(parentInfo.Dependencies, packageVersion) {
		return
	}

	parentInfo.Dependencies = append(parentInfo.Dependencies, packageVersion)
	i.Dependencies[*parent] = parentInfo
}

func (i *Installer) handleSemVerInstall(packageName *string, semVersion *string) models.PackageVersionMetadata {
	if *semVersion == "latest" {
		return i.fetchMetaDataWithCache(*packageName, "latest")
	}

	/*
		if the version is not latest than it has to be a constraint like ^1.1.0

		try to normalize constrainst if they are in the wrong format
		like >= 2.1.2 < 3.0.0 (semver throws a error here)
	*/
	re := regexp.MustCompile(`(>=|>|<=|<|=|~|\^)?\s*([\d]+\.[\d]+\.[\d]+)`)
	matches := re.FindAllString(*semVersion, -1)

	if len(matches) > 0 {
		*semVersion = strings.Join(matches, ", ")
	}

	constraint, err := semver.NewConstraint(*semVersion)
	if err != nil {
		fmt.Println("Unable to create constraint : ", *packageName, " : ", *semVersion)
		fmt.Println(err)
	}

	// check if a already resolved version of the package passes the required constraint
	i.resolvedMutex.RLock()
	resolvedPkgVersions, found := i.ResolvedMap[*packageName]
	i.resolvedMutex.RUnlock()

	if found {
		for _, v := range resolvedPkgVersions {
			if constraint.Check(v) {
				versionStr := v.String()
				return i.fetchMetaDataWithCache(*packageName, versionStr)
			}
		}
	}

	pkgFullMetaData := i.fetchFullMetaDataWithCache(*packageName)

	var versions []*semver.Version

	for raw := range pkgFullMetaData.Versions {
		version, err := semver.NewVersion(raw)
		if err != nil {
			fmt.Println("Unable to create version : ", *packageName, " : ", raw)
			fmt.Println(err)
		}

		versions = append(versions, version)
	}

	var finalVersion *semver.Version = nil
	for _, version := range versions {
		if constraint.Check(version) {
			if finalVersion == nil {
				finalVersion = version
			} else if version.GreaterThan(finalVersion) {
				finalVersion = version
			}
		}
	}

	return pkgFullMetaData.Versions[finalVersion.String()]
}

func (i *Installer) fetchMetaDataWithCache(packageName string, packageVersion string) models.PackageVersionMetadata {
	key := fmt.Sprintf("%s@%s", packageName, packageVersion)
	i.versionCacheMutex.RLock()

	if cached, ok := i.metaDataCache[key]; ok {
		i.versionCacheMutex.RUnlock()
		return cached
	}
	i.versionCacheMutex.RUnlock()

	metaData, err := i.httpHandler.FetchMetaDataWithVersion(packageName, packageVersion)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	i.versionCacheMutex.Lock()
	defer i.versionCacheMutex.Unlock()
	i.metaDataCache[key] = metaData
	return metaData
}

func (i *Installer) fetchFullMetaDataWithCache(packageName string) models.PackageMetadata {
	i.cacheMutex.RLock()
	if cached, ok := i.fullMetaDataCache[packageName]; ok {
		i.cacheMutex.RUnlock()
		return cached
	}
	i.cacheMutex.RUnlock()

	metaData, err := i.httpHandler.FetchFullMetaData(packageName)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	i.cacheMutex.Lock()
	defer i.cacheMutex.Unlock()
	i.fullMetaDataCache[packageName] = metaData
	return metaData
}

// Workers
func (i *Installer) enqueueResolveTask(name, version string, parent *string) {
	i.resolveWg.Add(1)
	i.resolveChan <- InstallTask{Name: name, Version: version, Parent: parent}
}

func (i *Installer) resolveWorker() {
	for task := range i.resolveChan {
		i.installPackage(task.Name, task.Version, task.Parent)
		i.resolveWg.Done()
	}
}

func (i *Installer) workerDownloader() {
	for task := range i.downloadChan {
		i.sem <- struct{}{}

		reader, err := i.httpHandler.DownloadTarBall(task.tarball)
		if err != nil {
			fmt.Println("Failed to download:", task.name)
			<-i.sem
			i.downloadWg.Done()
			continue
		}

		i.extractWg.Add(1)
		i.extractChan <- ExtractTask{
			Name:     task.name,
			Reader:   reader,
			DestPath: task.installPath,
		}

		<-i.sem

		i.downloadWg.Done()
	}
}

func (i *Installer) workerExtractor() {
	for task := range i.extractChan {
		err := i.httpHandler.InstallTarBall(task.Reader, task.DestPath)
		task.Reader.Close()
		if err != nil {
			fmt.Println("Failed to extract:", task.Name, err)
		}
		i.extractWg.Done()
	}
}
