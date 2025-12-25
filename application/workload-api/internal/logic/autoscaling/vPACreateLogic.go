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

type VPACreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建 VPA 资源
func NewVPACreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VPACreateLogic {
	return &VPACreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *VPACreateLogic) VPACreate(req *types.VPARequest) (resp string, err error) {
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

	// 获取目标资源的 UID 并设置 OwnerReference
	ownerUID, err := logic.GetOwnerResourceUID(l.ctx, client, versionDetail.Namespace,
		versionDetail.ResourceName, versionDetail.ResourceType, l.Logger)
	if err != nil {
		l.Errorf("获取目标资源 UID 失败: %v", err)
		return "", fmt.Errorf("获取目标资源 UID 失败: %v", err)
	}

	logic.SetOwnerReference(vpa, versionDetail.ResourceName,
		expectedTargetRef.Kind, expectedTargetRef.ApiVersion, ownerUID)
	l.Infof("已设置 OwnerReference: %s/%s", expectedTargetRef.Kind, versionDetail.ResourceName)

	// 注入注解
	utils.AddAnnotations(&vpa.ObjectMeta, &utils.AnnotationsInfo{
		ServiceName:   vpa.Name,
		ProjectName:   versionDetail.ProjectNameCn,
		WorkspaceName: versionDetail.WorkspaceNameCn,
		ProjectUuid:   versionDetail.ProjectUuid,
		OwnedByApp:    versionDetail.ApplicationNameCn,
	})

	// 创建 VPA 资源
	vpaOperator := client.VPA()
	createdVPA, err := vpaOperator.Create(vpa)
	if err != nil {
		l.Errorf("创建 VPA 资源失败: %v", err)

		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			VersionId:    versionDetail.VersionId,
			Title:        "创建 VPA",
			ActionDetail: fmt.Sprintf("创建 VPA 资源失败: %s, 错误: %v", vpa.Name, err),
			Status:       0,
		})

		return "", fmt.Errorf("创建 VPA 资源失败: %v", err)
	}

	// 记录成功的审计日志
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		VersionId:    versionDetail.VersionId,
		Title:        "创建 VPA",
		ActionDetail: fmt.Sprintf("创建 VPA 资源: %s", createdVPA.Name),
		Status:       1,
	})

	return fmt.Sprintf("VPA 资源 %s 创建成功", createdVPA.Name), nil
}
