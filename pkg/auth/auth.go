package auth

import (
	"fmt"
	"strings"

	"github.com/xenitab/azdo-git-proxy/pkg/config"
)

// IsPermitted checks if a specific user is permitted to access a path
func IsPermitted(c *config.Configuration, p string, t string) error {
	if p == "/"+c.Organization+"/"+"_apis" {
		return nil
	}

	comp := strings.Split(p, "/")
	if len(comp) < 5 {
		return fmt.Errorf("Path has to few components: %v", p)
	}

	org := comp[1]
	proj := comp[2]
	action := comp[3]

	var repo string
	if action == "_git" {
		repo = comp[4]
	} else if action == "_apis" {
		repo = comp[6]
	} else {
		return fmt.Errorf("Missing action path component: %v", p)
	}

	if c.Organization != org {
		return fmt.Errorf("Organization do not match: expected: %v, actual: %v, path: %v", c.Organization, org, p)
	}

	for _, repository := range c.Repositories {
		if repository.Project == proj && repository.Name == repo && repository.Token == t {
			return nil
		}
	}

	return fmt.Errorf("Could not find any matching configured repositories: %v", p)
}
