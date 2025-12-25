package utils

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ============ 基础信息 ============
	AnnotationServiceName = "ikubeops.com/service-name" // 中文名
	AnnotationProject     = "ikubeops.com/project"      // 所属项目
	AnnotationPlatform    = "ikubeops.com/platform"     // 创建平台
	AnnotationHomepage    = "ikubeops.com/homepage"     // 官网地址
	AnnotationApplication = "ikubeops.com/application"  // 所属服务
	// 所属应用
	AnnotationApplicationName = "ikubeops.com/owned-by-app"
	// ============ 操作信息 ============
	AnnotationCreatedBy = "ikubeops.com/created-by" // 创建者
	AnnotationUpdatedBy = "ikubeops.com/updated-by" // 更新者

	// ============ 业务信息 ============
	AnnotationDescription   = "ikubeops.com/description" // 描述
	AnnotationVersion       = "ikubeops.com/version"     // 版本
	AnnotationProjectUuid   = "ikubeops.com/project-uuid"
	AnnotationWorkspaceName = "ikubeops.com/workspace-name"
	AnnotationAuthor        = "ikubeops.com/author"
	AnnotationAuthorEmail   = "ikubeops.com/author-email"
	AnnotationAuthorWebsite = "ikubeops.com/website"
	// ============ 固定值 ============
	PlatformName    = "kube-nova"
	HomepageURL     = "www.ikubeops.com"
	PlatformWebsite = "https://www.ikubeops.com:8888"
	AuthorName      = "yanshicheng"
	AuthorEmail     = "ikubeops@163.com"
)

type AnnotationsInfo struct {
	ServiceName   string
	ProjectName   string
	Version       string
	Description   string
	ProjectUuid   string
	WorkspaceName string
	OwnedByApp    string
}

// addAnnotations 添加或更新注解（保留原有注解）
func AddAnnotations(meta *metav1.ObjectMeta, info *AnnotationsInfo) {
	if meta.Annotations == nil {
		meta.Annotations = make(map[string]string)
	}

	meta.Annotations[AnnotationAuthorWebsite] = PlatformWebsite
	meta.Annotations[AnnotationAuthor] = AuthorName
	meta.Annotations[AnnotationAuthorEmail] = AuthorEmail
	meta.Annotations[AnnotationPlatform] = PlatformName
	meta.Annotations[AnnotationServiceName] = info.ServiceName
	meta.Annotations[AnnotationProject] = info.ProjectName
	if info.Version != "" {
		meta.Annotations[AnnotationVersion] = info.Version
	}
	if info.OwnedByApp != "" {
		meta.Annotations[AnnotationApplicationName] = info.OwnedByApp
	}
	meta.Annotations[AnnotationDescription] = info.Description
	meta.Annotations[AnnotationHomepage] = HomepageURL
	meta.Annotations[AnnotationProjectUuid] = info.ProjectUuid
	meta.Annotations[AnnotationWorkspaceName] = info.WorkspaceName

}
