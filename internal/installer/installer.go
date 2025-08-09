package installer

import (
	"fmt"
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
	metaDataCache     sync.Map
	fullMetaDataChace sync.Map
	downloadCache     sync.Map
	mutex             sync.RWMutex
	httpHandler       *httphandler.HttpHandler

	taskChan     chan InstallTask
	downloadChan chan DownloadTask
	taskWg       sync.WaitGroup
	downloadWg   sync.WaitGroup

	sem chan struct{}
}

type PackageInstallInfo struct {
	name            string
	versionMetaData models.PackageVersionMetadata
	stringified     string
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

func NewInstaller() *Installer {
	installer := Installer{
		Dependencies: make(models.DependencyMap),
		ResolvedMap:  make(map[string][]*semver.Version),
		taskChan:     make(chan InstallTask, 1000),
		downloadChan: make(chan DownloadTask, 1000),
		sem:          make(chan struct{}, 32),
		httpHandler:  httphandler.NewHttpHandler(),
	}
	return &installer
}

// entry point for the installer
func (i *Installer) HandleInstall(packageName string) {
	workerCount := runtime.NumCPU()

	for w := 0; w < workerCount*8; w++ {
		go i.worker()
	}

	for w := 0; w < runtime.NumCPU()*10; w++ {
		go i.workerDownloader()
	}

	packageName, packageVersion := parser.ParseVersion(packageName)
	i.enqueue(packageName, packageVersion, nil)

	i.taskWg.Wait()
	close(i.taskChan)

	i.downloadWg.Wait()
	close(i.downloadChan)
}

func (i *Installer) enqueue(name, version string, parent *string) {
	i.taskWg.Add(1)
	i.taskChan <- InstallTask{Name: name, Version: version, Parent: parent}
}

func (i *Installer) worker() {
	for task := range i.taskChan {
		i.installPackage(task.Name, task.Version, task.Parent)
		i.taskWg.Done()
	}
}

func (i *Installer) workerDownloader() {
	for task := range i.downloadChan {
		i.sem <- struct{}{}

		// fmt.Println("From workerDownloader:", task.installPath)
		err := i.httpHandler.DownloadAndInstallTarBall(task.tarball, task.installPath)
		<-i.sem
		if err != nil {
			fmt.Println("Failed to install package : ", task.name)
		}
		i.downloadWg.Done()
	}
}

func (i *Installer) installPackage(packageName string, packageVersion string, parent *string) {
	fullVersion, _ := semver.NewVersion(packageVersion)
	var pkgInfo PackageInstallInfo
	var pkgMetadata models.PackageVersionMetadata

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

	pkgInfo.name = packageName
	pkgInfo.stringified = fmt.Sprintf("%s@%s", packageName, fullVersion.String())
	pkgInfo.versionMetaData = pkgMetadata

	if parent != nil {
		i.appendVersion(parent, pkgInfo.stringified)
	}

	// resolve dependencies recursively
	for depName, depVersion := range pkgInfo.versionMetaData.Dependencies {
		i.enqueue(depName, depVersion, &pkgInfo.name)
	}

	i.mutex.Lock()
	resolvedInfo, found := i.ResolvedMap[packageName]
	if !found {
		resolvedInfo = make([]*semver.Version, 0)
	}

	resolvedInfo = append(resolvedInfo, fullVersion)
	i.ResolvedMap[packageName] = resolvedInfo
	i.mutex.Unlock()

	key := pkgInfo.stringified
	if _, found := i.downloadCache.LoadOrStore(key, struct{}{}); found {
	} else {
		i.downloadWg.Add(1)
		i.downloadChan <- DownloadTask{
			name:        pkgInfo.name,
			tarball:     pkgInfo.versionMetaData.Dist.Tarball,
			installPath: filepath.Join("node_modules", packageName),
		}
	}

}

func (i *Installer) isResolved(packageInfo string) bool {
	// i.mutex.Lock()
	// defer i.mutex.Unlock()
	i.mutex.RLock()
	_, found := i.Dependencies[packageInfo]
	i.mutex.RUnlock()
	if found {
		return true
	}

	i.mutex.Lock()
	defer i.mutex.Unlock()
	i.Dependencies[packageInfo] = models.PackageInfo{
		Dependencies: []string{},
	}
	return false
}

// append version
func (i *Installer) appendVersion(parent *string, packageVersion string) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

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

func (i *Installer) fetchMetaDataWithCache(packageName string, packageVersion string) models.PackageVersionMetadata {
	key := fmt.Sprintf("%s@%s", packageName, packageVersion)
	if cached, ok := i.metaDataCache.Load(key); ok {
		return cached.(models.PackageVersionMetadata)
	}
	metaData := i.httpHandler.FetchMetaDataWithVersion(packageName, packageVersion)
	i.metaDataCache.Store(key, metaData)
	return metaData
}

func (i *Installer) fetchFullMetaDataWithCache(packageName string) models.PackageMetadata {
	if cached, ok := i.fullMetaDataChace.Load(packageName); ok {
		return cached.(models.PackageMetadata)
	}
	metaData := i.httpHandler.FetchFullMetaData(packageName)
	i.fullMetaDataChace.Store(packageName, metaData)
	return metaData
}

func (i *Installer) handleSemVerInstall(packageName *string, semVersion *string) models.PackageVersionMetadata {
	if *semVersion == "latest" {
		return i.fetchMetaDataWithCache(*packageName, "latest")
	}
	// if the version is not latest than it has to be a constraint like ^1.1.0

	// try to normalize constrainst if they are in the wrong format
	// like >= 2.1.2 < 3.0.0 (semver throws a error here)
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
	i.mutex.RLock()
	resolvedPkgVersions, found := i.ResolvedMap[*packageName]
	i.mutex.RUnlock()

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
