package models

import "fmt"

type PackageVersionMetadata struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Dist    struct {
		Tarball string `json:"tarball"`
	} `json:"dist"`
	Dependencies map[string]string `json:"dependencies"`
}

type PackageMetadata struct {
	DistTags struct {
		Latest string `json:"latest"`
	} `json:"dist-tags"`
	Versions map[string]PackageVersionMetadata
}

type PackageInfo struct {
	Dependencies []string
}

type DependencyMap map[string]PackageInfo

func DebugMap(mp DependencyMap) {
	for pkg, deps := range mp {
		fmt.Println(pkg, ":")
		for _, dep := range deps.Dependencies {
			fmt.Println("\t", dep)
		}
	}
}

func (v PackageVersionMetadata) String() string {
	return fmt.Sprintf("%s\n%s\n%+v\n%+v\n", v.Name, v.Version, v.Dist, v.Dependencies)
}
