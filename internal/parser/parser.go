package parser

import (
	"strings"
)

func ParseVersion(packageName string) (string, string) {
	var packageData []string
	// pacage names may start with @ for organisation
	if packageName[0] == '@' {
		packageData = strings.Split(packageName, "@")
		if len(packageData) == 2 {
			return "@" + packageData[1], "latest"
		}
		return ("@" + packageData[1]), packageData[2]
	}
	packageData = strings.Split(packageName, "@")
	if len(packageData) == 1 {
		return packageData[0], "latest"
	}
	return packageData[0], packageData[1]
}
