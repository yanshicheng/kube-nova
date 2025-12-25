package autoscaling

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/logic"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	autoscalingv2 "k8s.io/api/autoscaling/v2"

	"github.com/zeromicro/go-zero/core/logx"
)

type HPAUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新 HPA 资源
func NewHPAUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HPAUpdateLogic {
	return &HPAUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *HPAUpdateLogic) HPAUpdate(req *types.HPARequest) (resp string, err error) {
	// 获取集群客户端和版本详情
	client, versionDetail, err := logic.GetClusterClientAndVersion(l.ctx, l.svcCtx, req.VersionId, l.Logger)
	if err != nil {
		return "", err
	}

	// 解析 YAML
	hpa := &autoscalingv2.HorizontalPodAutoscaler{}
	if err := logic.ParseYAMLToObject(req.HpaYamlStr, hpa); err != nil {
		l.Errorf("解析 HPA YAML 失败: %v", err)
		return "", fmt.Errorf("解析 HPA YAML 失败: %v", err)
	}

	// 构建预期的 TargetRef
	expectedTargetRef := logic.GetTargetRefFromVersion(versionDetail)

	// 验证资源
	validator := &logic.ResourceValidator{
		ExpectedNamespace: versionDetail.Namespace,
		ExpectedTargetRef: expectedTargetRef,
		Logger:            l.Logger,
	}
	if err := validator.ValidateHPA(hpa); err != nil {
		l.Errorf("HPA 资源验证失败: %v", err)
		return "", fmt.Errorf("资源验证失败: %v", err)
	}

	// 获取现有的 HPA 资源
	hpaOperator := client.HPA()
	existingHPA, err := hpaOperator.Get(versionDetail.Namespace, hpa.Name)
	if err != nil {
		l.Errorf("获取现有 HPA 资源失败: %v", err)
		return "", fmt.Errorf("获取现有 HPA 资源失败: %v", err)
	}

	// 保留必要的元数据
	hpa.ResourceVersion = existingHPA.ResourceVersion
	hpa.UID = existingHPA.UID
	hpa.CreationTimestamp = existingHPA.CreationTimestamp

	// 保留现有的 OwnerReferences（如果存在）
	if len(existingHPA.OwnerReferences) > 0 {
		hpa.OwnerReferences = existingHPA.OwnerReferences
	} else {
		// 如果不存在，设置新的 OwnerReference
		ownerUID, err := logic.GetOwnerResourceUID(l.ctx, client, versionDetail.Namespace,
			versionDetail.ResourceName, versionDetail.ResourceType, l.Logger)
		if err != nil {
			l.Errorf("获取目标资源 UID 失败，跳过 OwnerReference 设置: %v", err)
		} else {
			logic.SetOwnerReference(hpa, versionDetail.ResourceName,
				expectedTargetRef.Kind, expectedTargetRef.ApiVersion, ownerUID)
		}
	}

	// 注入注解
	utils.AddAnnotations(&hpa.ObjectMeta, &utils.AnnotationsInfo{
		ServiceName:   hpa.Name,
		ProjectName:   versionDetail.ProjectNameCn,
		WorkspaceName: versionDetail.WorkspaceNameCn,
		ProjectUuid:   versionDetail.ProjectUuid,
		OwnedByApp:    versionDetail.ApplicationNameCn,
	})

	// 更新 HPA 资源
	updatedHPA, err := hpaOperator.Update(hpa)
	if err != nil {
		l.Errorf("更新 HPA 资源失败: %v", err)

		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			VersionId:    versionDetail.VersionId,
			Title:        "更新 HPA",
			ActionDetail: fmt.Sprintf("更新 HPA 资源失败: %s, 错误: %v", hpa.Name, err),
			Status:       0,
		})

		return "", fmt.Errorf("更新 HPA 资源失败: %v", err)
	}

	// 记录成功的审计日志
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		VersionId:    versionDetail.VersionId,
		Title:        "更新 HPA",
		ActionDetail: fmt.Sprintf("更新 HPA 资源: %s", updatedHPA.Name),
		Status:       1,
	})

	return fmt.Sprintf("HPA 资源 %s 更新成功", updatedHPA.Name), nil
}
