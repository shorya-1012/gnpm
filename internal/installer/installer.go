package installer

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/shorya-1012/gnpm/internal/http_handler"
	"github.com/shorya-1012/gnpm/internal/models"
	"github.com/shorya-1012/gnpm/internal/parser"
)

type Installer struct {
	Dependencies models.DependencyMap
}

type PackageInstallInfo struct {
	name            string
	versionMetaData models.PackageVersionMetadata
	stringified     string
}

func (i *Installer) HandleInstall(packageName string) {
	packageName, packageVersion := parser.ParseVersion(packageName)
	i.installPackage(packageName, packageVersion, nil)
}

func (i *Installer) installPackage(packageName string, packageVersion string, parent *string) PackageInstallInfo {
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

			return PackageInstallInfo{}
		}

		pkgMetadata = httphandler.FetchMetaDataWithVersion(packageName, fullVersion.String())
	}

	pkgInfo.name = packageName
	pkgInfo.stringified = fmt.Sprintf("%s@%s", packageName, fullVersion.String())
	pkgInfo.versionMetaData = pkgMetadata

	if parent != nil {
		i.appendVersion(parent, pkgInfo.stringified)
	}

	i.resolveDependencies(&pkgInfo)

	// fmt.Println("Installed ", packageName, fullVersion)
	return pkgInfo
}

func (i *Installer) isResolved(packageInfo string) bool {
	_, found := i.Dependencies[packageInfo]
	if found {
		return true
	}

	// if it doesn't exist then add it to the fuckAss Map;
	i.Dependencies[packageInfo] = models.PackageInfo{
		Dependencies: []string{},
	}
	return false
}

// append version
func (i *Installer) appendVersion(parent *string, packageVersion string) {
	parentInfo, found := i.Dependencies[*parent]
	if !found {
		parentInfo = models.PackageInfo{
			Dependencies: []string{},
		}
	}

	parentInfo.Dependencies = append(parentInfo.Dependencies, packageVersion)
	i.Dependencies[*parent] = parentInfo
}

// resolve Dependencies
func (i *Installer) resolveDependencies(pkgInfo *PackageInstallInfo) {
	for dependencyName, dependencyVersion := range pkgInfo.versionMetaData.Dependencies {
		i.installPackage(dependencyName, dependencyVersion, &pkgInfo.name)
	}
}

func (i *Installer) handleSemVerInstall(packageName *string, semVersion *string) models.PackageVersionMetadata {
	pkgFullMetaData := httphandler.FetchFullMetaData(*packageName)
	if *semVersion == "latest" {
		return pkgFullMetaData.Versions[pkgFullMetaData.DistTags.Latest]
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
