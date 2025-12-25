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

type CanaryCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建 Canary 资源
func NewCanaryCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CanaryCreateLogic {
	return &CanaryCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CanaryCreateLogic) CanaryCreate(req *types.CreateCanaryRequest) (resp string, err error) {
	// 获取集群客户端和版本详情
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

	// 获取目标资源的 UID 并设置 OwnerReference
	ownerUID, err := logic.GetOwnerResourceUID(l.ctx, client, versionDetail.Namespace,
		versionDetail.ResourceName, versionDetail.ResourceType, l.Logger)
	if err != nil {
		l.Errorf("获取目标资源 UID 失败: %v", err)
		return "", fmt.Errorf("获取目标资源 UID 失败: %v", err)
	}

	logic.SetOwnerReference(canary, versionDetail.ResourceName,
		expectedTargetRef.Kind, expectedTargetRef.ApiVersion, ownerUID)
	l.Infof("已设置 OwnerReference: %s/%s", expectedTargetRef.Kind, versionDetail.ResourceName)

	// 注入注解
	utils.AddAnnotations(&canary.ObjectMeta, &utils.AnnotationsInfo{
		ServiceName:   canary.Name,
		ProjectName:   versionDetail.ProjectNameCn,
		WorkspaceName: versionDetail.WorkspaceNameCn,
		ProjectUuid:   versionDetail.ProjectUuid,
		OwnedByApp:    versionDetail.ApplicationNameCn,
	})

	// 创建 Canary 资源
	canaryOperator := client.Flagger()
	createdCanary, err := canaryOperator.Create(canary)
	if err != nil {
		l.Errorf("创建 Canary 资源失败: %v", err)

		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			VersionId:    versionDetail.VersionId,
			Title:        "创建 Canary",
			ActionDetail: fmt.Sprintf("创建 Canary 资源失败: %s, 错误: %v", canary.Name, err),
			Status:       0,
		})

		return "", fmt.Errorf("创建 Canary 资源失败: %v", err)
	}

	// 记录成功的审计日志
	_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		VersionId:    versionDetail.VersionId,
		Title:        "创建 Canary",
		ActionDetail: fmt.Sprintf("创建 Canary 资源: %s", createdCanary.Name),
		Status:       1,
	})

	return fmt.Sprintf("Canary 资源 %s 创建成功", createdCanary.Name), nil
}
