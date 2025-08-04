package installer

import (
	"github.com/shorya-1012/gnpm/internal/models"
)

type Installer struct {
	DependencyMap map[string]models.PackageLock
}

// resolve the version to install
// install the root
