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

type HPACreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建 HPA 资源
func NewHPACreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HPACreateLogic {
	return &HPACreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *HPACreateLogic) HPACreate(req *types.HPARequest) (resp string, err error) {
	client, versionDetail, err := logic.GetClusterClientAndVersion(l.ctx, l.svcCtx, req.VersionId, l.Logger)
	if err != nil {
		return "", err
	}

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

	// 获取目标资源的 UID 并设置 OwnerReference
	ownerUID, err := logic.GetOwnerResourceUID(l.ctx, client, versionDetail.Namespace,
		versionDetail.ResourceName, versionDetail.ResourceType, l.Logger)
	if err != nil {
		l.Errorf("获取目标资源 UID 失败: %v", err)
		return "", fmt.Errorf("获取目标资源 UID 失败: %v", err)
	}

	logic.SetOwnerReference(hpa, versionDetail.ResourceName,
		expectedTargetRef.Kind, expectedTargetRef.ApiVersion, ownerUID)

	// 注入注解
	utils.AddAnnotations(&hpa.ObjectMeta, &utils.AnnotationsInfo{
		ServiceName:   hpa.Name,
		ProjectName:   versionDetail.ProjectNameCn,
		WorkspaceName: versionDetail.WorkspaceNameCn,
		ProjectUuid:   versionDetail.ProjectUuid,
		OwnedByApp:    versionDetail.ApplicationNameCn,
	})

	// 创建 HPA 资源
	hpaOperator := client.HPA()
	createdHPA, err := hpaOperator.Create(hpa)
	if err != nil {
		l.Errorf("创建 HPA 资源失败: %v", err)

		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			VersionId:    versionDetail.VersionId,
			Title:        "创建 HPA",
			ActionDetail: fmt.Sprintf("创建 HPA 资源失败: %s, 错误: %v", hpa.Name, err),
			Status:       0,
		})

		return "", fmt.Errorf("创建 HPA 资源失败: %v", err)
	}

	// 记录成功的审计日志
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		VersionId:    versionDetail.VersionId,
		Title:        "创建 HPA",
		ActionDetail: fmt.Sprintf("创建 HPA 资源: %s", createdHPA.Name),
		Status:       1,
	})

	return fmt.Sprintf("HPA 资源 %s 创建成功", createdHPA.Name), nil
}
