package operator

import "strings"

// identifyFlaggerResource 识别 Flagger 资源
func (s *ClusterResourceSync) identifyFlaggerResource(
	resourceName string,
	annotations map[string]string,
	labels map[string]string,
) *FlaggerResourceInfo {
	info := &FlaggerResourceInfo{
		IsFlaggerManaged: false,
		OriginalAppName:  resourceName,
		VersionRole:      VersionRoleStable,
		ParentName:       "",
	}

	// Label 识别
	if labels != nil {
		if managedBy, ok := labels[FlaggerManagedByLabel]; ok && managedBy == "flagger" {
			info.IsFlaggerManaged = true
			if appName, ok := labels[FlaggerAppNameLabel]; ok && appName != "" {
				info.OriginalAppName = appName
			}
		}
	}

	// Annotation 识别
	if annotations != nil {
		if _, ok := annotations[FlaggerPrimaryAnnotation]; ok {
			info.VersionRole = VersionRolePrimary
			info.IsFlaggerManaged = true
		} else if _, ok := annotations[FlaggerCanaryAnnotation]; ok {
			info.VersionRole = VersionRoleCanary
			info.IsFlaggerManaged = true
		}
	}

	// 命名规则推断
	if !info.IsFlaggerManaged {
		if strings.HasSuffix(resourceName, "-primary") {
			info.IsFlaggerManaged = true
			info.OriginalAppName = strings.TrimSuffix(resourceName, "-primary")
			info.VersionRole = VersionRolePrimary
		} else if strings.HasSuffix(resourceName, "-canary") {
			info.IsFlaggerManaged = true
			info.OriginalAppName = strings.TrimSuffix(resourceName, "-canary")
			info.VersionRole = VersionRoleCanary
		} else if strings.HasSuffix(resourceName, "-blue") {
			info.IsFlaggerManaged = true
			info.OriginalAppName = strings.TrimSuffix(resourceName, "-blue")
			info.VersionRole = VersionRoleBlue
		} else if strings.HasSuffix(resourceName, "-green") {
			info.IsFlaggerManaged = true
			info.OriginalAppName = strings.TrimSuffix(resourceName, "-green")
			info.VersionRole = VersionRoleGreen
		}
	}

	// 设置 ParentName（用于关联 primary 和 canary）
	if info.IsFlaggerManaged {
		parentName := resourceName
		// 去掉角色后缀
		if strings.HasSuffix(parentName, "-primary") {
			parentName = strings.TrimSuffix(parentName, "-primary")
		} else if strings.HasSuffix(parentName, "-canary") {
			parentName = strings.TrimSuffix(parentName, "-canary")
		} else if strings.HasSuffix(parentName, "-blue") {
			parentName = strings.TrimSuffix(parentName, "-blue")
		} else if strings.HasSuffix(parentName, "-green") {
			parentName = strings.TrimSuffix(parentName, "-green")
		}
		info.ParentName = parentName
	}

	return info
}
