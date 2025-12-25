package autoscaling

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/logic"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"

	"github.com/zeromicro/go-zero/core/logx"
)

type VPAUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新 VPA 资源
func NewVPAUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VPAUpdateLogic {
	return &VPAUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *VPAUpdateLogic) VPAUpdate(req *types.VPARequest) (resp string, err error) {
	// 获取集群客户端和版本详情
	client, versionDetail, err := logic.GetClusterClientAndVersion(l.ctx, l.svcCtx, req.VersionId, l.Logger)
	if err != nil {
		return "", err
	}

	// 解析 YAML
	vpa := &vpav1.VerticalPodAutoscaler{}
	if err := logic.ParseYAMLToObject(req.VpaYamlStr, vpa); err != nil {
		l.Errorf("解析 VPA YAML 失败: %v", err)
		return "", fmt.Errorf("解析 VPA YAML 失败: %v", err)
	}

	// 构建预期的 TargetRef
	expectedTargetRef := logic.GetTargetRefFromVersion(versionDetail)

	// 验证资源
	validator := &logic.ResourceValidator{
		ExpectedNamespace: versionDetail.Namespace,
		ExpectedTargetRef: expectedTargetRef,
		Logger:            l.Logger,
	}
	if err := validator.ValidateVPA(vpa); err != nil {
		l.Errorf("VPA 资源验证失败: %v", err)
		return "", fmt.Errorf("资源验证失败: %v", err)
	}

	// 获取现有的 VPA 资源
	vpaOperator := client.VPA()
	existingVPA, err := vpaOperator.Get(versionDetail.Namespace, vpa.Name)
	if err != nil {
		l.Errorf("获取现有 VPA 资源失败: %v", err)
		return "", fmt.Errorf("获取现有 VPA 资源失败: %v", err)
	}

	// 保留必要的元数据
	vpa.ResourceVersion = existingVPA.ResourceVersion
	vpa.UID = existingVPA.UID
	vpa.CreationTimestamp = existingVPA.CreationTimestamp

	// 保留现有的 OwnerReferences（如果存在）
	if len(existingVPA.OwnerReferences) > 0 {
		vpa.OwnerReferences = existingVPA.OwnerReferences
	} else {
		// 如果不存在，设置新的 OwnerReference
		ownerUID, err := logic.GetOwnerResourceUID(l.ctx, client, versionDetail.Namespace,
			versionDetail.ResourceName, versionDetail.ResourceType, l.Logger)
		if err != nil {
			l.Errorf("获取目标资源 UID 失败，跳过 OwnerReference 设置: %v", err)
		} else {
			logic.SetOwnerReference(vpa, versionDetail.ResourceName,
				expectedTargetRef.Kind, expectedTargetRef.ApiVersion, ownerUID)
		}
	}

	// 注入注解
	utils.AddAnnotations(&vpa.ObjectMeta, &utils.AnnotationsInfo{
		ServiceName:   vpa.Name,
		ProjectName:   versionDetail.ProjectNameCn,
		WorkspaceName: versionDetail.WorkspaceNameCn,
		ProjectUuid:   versionDetail.ProjectUuid,
		OwnedByApp:    versionDetail.ApplicationNameCn,
	})

	// 更新 VPA 资源
	updatedVPA, err := vpaOperator.Update(vpa)
	if err != nil {
		l.Errorf("更新 VPA 资源失败: %v", err)

		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			VersionId:    versionDetail.VersionId,
			Title:        "更新 VPA",
			ActionDetail: fmt.Sprintf("更新 VPA 资源失败: %s, 错误: %v", vpa.Name, err),
			Status:       0,
		})

		return "", fmt.Errorf("更新 VPA 资源失败: %v", err)
	}

	// 记录成功的审计日志
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		VersionId:    versionDetail.VersionId,
		Title:        "更新 VPA",
		ActionDetail: fmt.Sprintf("更新 VPA 资源: %s", updatedVPA.Name),
		Status:       1,
	})

	return fmt.Sprintf("VPA 资源 %s 更新成功", updatedVPA.Name), nil
}
