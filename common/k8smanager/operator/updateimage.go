package operator

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
)

// validateImageFormat 验证镜像格式
func validateImageFormat(image string) error {
	if image == "" {
		return fmt.Errorf("镜像不能为空")
	}

	// 基本格式检查
	// 合法格式: nginx, nginx:1.21, registry.example.com/nginx:1.21, registry.example.com:5000/path/nginx:1.21@sha256:xxx
	if strings.ContainsAny(image, " \t\n") {
		return fmt.Errorf("镜像名称不能包含空白字符")
	}

	// 检查是否有非法字符
	for _, r := range image {
		if r > 127 {
			return fmt.Errorf("镜像名称只能包含ASCII字符")
		}
	}

	return nil
}

// extractImageTag 提取镜像的简短标识（用于日志）
func extractImageTag(image string) string {
	// 处理 digest 格式: image@sha256:xxx
	if idx := strings.LastIndex(image, "@"); idx != -1 {
		digest := image[idx+1:]
		if len(digest) > 15 {
			digest = digest[:15] + "..."
		}
		return digest
	}

	// 处理 tag 格式: image:tag
	if idx := strings.LastIndex(image, ":"); idx != -1 {
		// 排除端口号的情况 (registry:5000/image)
		afterColon := image[idx+1:]
		if !strings.Contains(afterColon, "/") {
			return afterColon
		}
	}

	return "latest"
}

// getAvailableContainerNames 获取可用的容器名列表
func (d *deploymentOperator) getAvailableContainerNames(deployment *appsv1.Deployment) []string {
	names := make([]string, 0)

	for _, c := range deployment.Spec.Template.Spec.InitContainers {
		names = append(names, fmt.Sprintf("init:%s", c.Name))
	}

	for _, c := range deployment.Spec.Template.Spec.Containers {
		names = append(names, c.Name)
	}

	return names
}
