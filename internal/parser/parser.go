package parser

import (
	"github.com/Masterminds/semver"
	"strings"
)

func ParseVersion(packageName string) (string, string) {
	packageData := strings.Split(packageName, "@")
	if len(packageData) == 1 {
		return packageData[0], "latest"
	}
	return packageData[0], packageData[1]
}

func IsFullVersion(version string) *semver.Version {
	v, _ := semver.NewVersion(version)
	return v
}
