package canary

import (
	"context"
	"fmt"

	flaggerv1 "github.com/fluxcd/flagger/pkg/apis/flagger/v1beta1"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/logic"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type CanaryUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新 Canary 资源
func NewCanaryUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CanaryUpdateLogic {
	return &CanaryUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CanaryUpdateLogic) CanaryUpdate(req *types.UpdateCanaryRequest) (resp string, err error) {
	client, versionDetail, err := logic.GetClusterClientAndVersion(l.ctx, l.svcCtx, req.VersionId, l.Logger)
	if err != nil {
		return "", err
	}

	// 解析 YAML
	canary := &flaggerv1.Canary{}
	if err := logic.ParseYAMLToObject(req.FlaggerYamlStr, canary); err != nil {
		l.Errorf("解析 Canary YAML 失败: %v", err)
		return "", fmt.Errorf("解析 Canary YAML 失败: %v", err)
	}

	// 验证名称是否匹配
	if canary.Name != req.Name {
		return "", fmt.Errorf("YAML 中的 Canary 名称 %s 与请求名称 %s 不匹配", canary.Name, req.Name)
	}

	// 构建预期的 TargetRef
	expectedTargetRef := logic.GetTargetRefFromVersion(versionDetail)

	// 验证资源
	validator := &logic.ResourceValidator{
		ExpectedNamespace: versionDetail.Namespace,
		ExpectedTargetRef: expectedTargetRef,
		Logger:            l.Logger,
	}
	if err := validator.ValidateCanary(canary); err != nil {
		l.Errorf("Canary 资源验证失败: %v", err)
		return "", fmt.Errorf("资源验证失败: %v", err)
	}

	// 获取现有的 Canary 资源
	canaryOperator := client.Flagger()
	existingCanary, err := canaryOperator.Get(versionDetail.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取现有 Canary 资源失败: %v", err)
		return "", fmt.Errorf("获取现有 Canary 资源失败: %v", err)
	}

	// 保留必要的元数据
	canary.ResourceVersion = existingCanary.ResourceVersion
	canary.UID = existingCanary.UID
	canary.CreationTimestamp = existingCanary.CreationTimestamp

	// 保留现有的 OwnerReferences
	if len(existingCanary.OwnerReferences) > 0 {
		canary.OwnerReferences = existingCanary.OwnerReferences
	} else {
		// 如果不存在，设置新的 OwnerReference
		ownerUID, err := logic.GetOwnerResourceUID(l.ctx, client, versionDetail.Namespace,
			versionDetail.ResourceName, versionDetail.ResourceType, l.Logger)
		if err != nil {
			l.Errorf("获取目标资源 UID 失败，跳过 OwnerReference 设置: %v", err)
		} else {
			logic.SetOwnerReference(canary, versionDetail.ResourceName,
				expectedTargetRef.Kind, expectedTargetRef.ApiVersion, ownerUID)
		}
	}

	// 注入注解
	utils.AddAnnotations(&canary.ObjectMeta, &utils.AnnotationsInfo{
		ServiceName:     canary.Name,
		ApplicationEn:   versionDetail.ApplicationNameEn,
		ApplicationName: versionDetail.ApplicationNameCn,
		ProjectName:     versionDetail.ProjectNameCn,
		WorkspaceName:   versionDetail.WorkspaceNameCn,
		ProjectUuid:     versionDetail.ProjectUuid,
		OwnedByApp:      versionDetail.ApplicationNameCn,
	})

	// 更新 Canary 资源
	updatedCanary, err := canaryOperator.Update(canary)
	if err != nil {
		l.Errorf("更新 Canary 资源失败: %v", err)

		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			VersionId:    versionDetail.VersionId,
			Title:        "更新 Canary",
			ActionDetail: fmt.Sprintf("更新 Canary 资源失败: %s, 错误: %v", canary.Name, err),
			Status:       0,
		})

		return "", fmt.Errorf("更新 Canary 资源失败: %v", err)
	}

	// 记录成功的审计日志
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		VersionId:    versionDetail.VersionId,
		Title:        "更新 Canary",
		ActionDetail: fmt.Sprintf("更新 Canary 资源: %s", updatedCanary.Name),
		Status:       1,
	})

	l.Infof("Canary 资源更新成功: %s", updatedCanary.Name)
	return fmt.Sprintf("Canary 资源 %s 更新成功", updatedCanary.Name), nil
}
