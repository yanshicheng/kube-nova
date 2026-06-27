// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type BatchCreateApplicationResourceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 批量导入 YAML 创建应用资源
func NewBatchCreateApplicationResourceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchCreateApplicationResourceLogic {
	return &BatchCreateApplicationResourceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchCreateApplicationResourceLogic) BatchCreateApplicationResource(req *types.BatchCreateApplicationResource) (resp string, err error) {
	username, _ := l.ctx.Value("username").(string)
	if username == "" {
		username = "system"
	}
	roles, _ := l.ctx.Value("roles").([]string)

	namespaceDetail, err := l.svcCtx.ManagerRpc.GetClusterNsDetail(l.ctx, &managerservice.GetClusterNsDetailReq{
		ClusterUuid: req.ClusterUuid,
		Namespace:   req.Namespace,
	})
	if err != nil {
		l.Errorf("获取命名空间详情失败: %v", err)
		return "", fmt.Errorf("获取命名空间详情失败: %v", err)
	}

	documents, err := parseBatchCreateDocuments(req.YamlStr)
	if err != nil {
		return "", err
	}
	if err := normalizeBatchDocuments(documents, req.Namespace); err != nil {
		return "", err
	}

	hasClusterScopedResource := false
	for _, document := range documents {
		if document.ClusterScoped {
			hasClusterScopedResource = true
			break
		}
	}
	if hasClusterScopedResource && !hasSuperAdminRole(roles) {
		return "", fmt.Errorf("仅 super_admin 可创建集群级资源")
	}

	applicationMeta, err := buildBatchApplicationMeta(documents)
	if err != nil {
		return "", err
	}

	annotationInfo := &utils.AnnotationsInfo{
		ProjectName:   namespaceDetail.ProjectNameCn,
		ProjectUuid:   namespaceDetail.ProjectUuid,
		WorkspaceName: namespaceDetail.WorkspaceNameCn,
	}
	if applicationMeta != nil {
		annotationInfo.ServiceName = applicationMeta.ResourceName
		annotationInfo.ApplicationName = applicationMeta.NameCn
		annotationInfo.ApplicationEn = applicationMeta.NameEn
		annotationInfo.Version = applicationMeta.Version
		annotationInfo.Description = applicationMeta.Description
	}
	applyBatchAnnotations(documents, annotationInfo)

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	createdDocuments := make([]*batchResourceDocument, 0, len(documents))
	for _, document := range documents {
		if createErr := createBatchResource(client, document); createErr != nil {
			rollbackErrors := rollbackBatchResources(client, createdDocuments)
			failDetail := fmt.Sprintf(
				"用户 %s 批量创建资源失败，失败资源: %s/%s，错误: %v",
				username,
				document.Kind,
				document.Name,
				createErr,
			)
			if len(rollbackErrors) > 0 {
				failDetail = failDetail + "，回滚结果: " + strings.Join(rollbackErrors, "; ")
			}
			_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
				WorkspaceId:  namespaceDetail.WorkspaceId,
				ClusterUuid:  req.ClusterUuid,
				Title:        "YAML 批量创建资源",
				ActionDetail: failDetail,
				Status:       0,
			})
			return "", fmt.Errorf("创建资源 %s/%s 失败: %v", document.Kind, document.Name, createErr)
		}
		createdDocuments = append(createdDocuments, document)
	}

	var versionId uint64
	var applicationId uint64
	if applicationMeta != nil && !applicationMeta.OneTime {
		addApplicationResp, addApplicationErr := l.svcCtx.ManagerRpc.ApplicationAdd(l.ctx, &managerservice.AddOnecProjectApplicationReq{
			WorkspaceId:  namespaceDetail.WorkspaceId,
			NameCn:       applicationMeta.NameCn,
			NameEn:       applicationMeta.NameEn,
			ResourceType: applicationMeta.ResourceType,
			Description:  applicationMeta.Description,
			CreatedBy:    username,
			UpdatedBy:    username,
		})
		if addApplicationErr != nil {
			rollbackErrors := rollbackBatchResources(client, createdDocuments)
			return "", fmt.Errorf("创建应用记录失败: %v，已回滚资源: %s", addApplicationErr, strings.Join(rollbackErrors, "; "))
		}
		applicationId = addApplicationResp.Id

		addVersionResp, addVersionErr := l.svcCtx.ManagerRpc.VersionAdd(l.ctx, &managerservice.AddOnecProjectVersionReq{
			ApplicationId: addApplicationResp.Id,
			Version:       applicationMeta.Version,
			ResourceName:  applicationMeta.ResourceName,
			CreatedBy:     username,
			UpdatedBy:     username,
		})
		if addVersionErr != nil {
			rollbackErrors := rollbackBatchResources(client, createdDocuments)
			_, _ = l.svcCtx.ManagerRpc.ApplicationDel(l.ctx, &managerservice.DelOnecProjectApplicationReq{
				Id: addApplicationResp.Id,
			})
			return "", fmt.Errorf("创建版本记录失败: %v，已回滚资源: %s", addVersionErr, strings.Join(rollbackErrors, "; "))
		}
		versionId = addVersionResp.Id
	}

	successDetail := fmt.Sprintf(
		"用户 %s 通过 YAML 批量创建 %d 个资源: %s",
		username,
		len(createdDocuments),
		summarizeBatchResources(createdDocuments),
	)
	if applicationMeta != nil {
		successDetail = fmt.Sprintf(
			"%s，应用: %s(%s)，版本: %s",
			successDetail,
			applicationMeta.NameCn,
			applicationMeta.NameEn,
			applicationMeta.Version,
		)
	}

	auditReq := &managerservice.AddOnecProjectAuditLogReq{
		WorkspaceId:  namespaceDetail.WorkspaceId,
		ClusterUuid:  req.ClusterUuid,
		Title:        "YAML 批量创建资源",
		ActionDetail: successDetail,
		Status:       1,
	}
	if versionId > 0 {
		auditReq.VersionId = versionId
	} else if applicationId > 0 {
		auditReq.ApplicationId = applicationId
	}
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, auditReq)

	return fmt.Sprintf("批量创建成功，共创建 %d 个资源", len(createdDocuments)), nil
}

func hasSuperAdminRole(roles []string) bool {
	for _, role := range roles {
		if strings.EqualFold(role, "super_admin") || strings.EqualFold(role, "SUPER_ADMIN") {
			return true
		}
	}
	return false
}
