package models

import "fmt"

type PackageVersionData struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Dist    struct {
		Tarball string `json:"tarball"`
	} `json:"dist"`
	Dependencies map[string]string `json:"dependencies"`
}

type PackageData struct {
	DistTags struct {
		Latest string `json:"latest"`
	} `json:"dist-tags"`
	Versions map[string]PackageVersionData
}

type PackageLock struct {
	Dependencies []string
}

// debug
func (v PackageVersionData) String() string {
	return fmt.Sprintf("%s\n%s\n%+v\n%+v\n", v.Name, v.Version, v.Dist, v.Dependencies)
}
