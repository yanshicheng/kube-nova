package tekton

import (
	"regexp"
	"strings"
)

var nameCleanPattern = regexp.MustCompile(`[^a-z0-9-]+`)

func ProjectNamespace(projectCode string) string {
	name := strings.ToLower(strings.TrimSpace(projectCode))
	name = nameCleanPattern.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if len(name) > 56 {
		name = strings.Trim(name[:56], "-")
	}
	if name == "" {
		name = "project"
	}
	return "devops-" + name
}

func ResourceName(values ...string) string {
	value := strings.ToLower(strings.Join(values, "-"))
	value = nameCleanPattern.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		value = "pipeline"
	}
	if len(value) > 63 {
		value = strings.Trim(value[:63], "-")
	}
	return value
}
