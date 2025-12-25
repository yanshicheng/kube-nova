package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/yanshicheng/kube-nova/common/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/zeromicro/go-zero/core/logx"
)

type SecretUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新 Secret
func NewSecretUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SecretUpdateLogic {
	return &SecretUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SecretUpdateLogic) SecretUpdate(req *types.SecretRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return "", fmt.Errorf("获取项目工作空间详情失败")
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	secretClient := client.Secrets()

	var secret corev1.Secret
	err = yaml.Unmarshal([]byte(req.SecretYamlStr), &secret)
	if err != nil {
		l.Errorf("解析 Secret YAML 失败: %v", err)
		return "", fmt.Errorf("解析 Secret YAML 失败")
	}

	// 设置命名空间
	if secret.Namespace == "" {
		secret.Namespace = workloadInfo.Data.Namespace
	}
	// 获取项目详情
	projectDetail, err := l.svcCtx.ManagerRpc.GetClusterNsDetail(l.ctx, &managerservice.GetClusterNsDetailReq{
		ClusterUuid: workloadInfo.Data.ClusterUuid,
		Namespace:   workloadInfo.Data.Namespace,
	})
	if err != nil {
		l.Errorf("获取项目详情失败: %v", err)
		return "", fmt.Errorf("获取项目详情失败")
	} else {
		// 注入注解
		utils.AddAnnotations(&secret.ObjectMeta, &utils.AnnotationsInfo{
			ServiceName:   secret.Name,
			ProjectName:   projectDetail.ProjectNameCn,
			WorkspaceName: projectDetail.WorkspaceNameCn,
			ProjectUuid:   projectDetail.ProjectUuid,
		})
	}
	// 更新 Secret
	_, updateErr := secretClient.Update(&secret)
	if updateErr != nil {
		l.Errorf("更新 Secret 失败: %v", updateErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			WorkspaceId:  req.WorkloadId,
			Title:        "更新 Secret",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 更新 Secret %s 失败, 类型: %s, 错误原因: %v", username, secret.Namespace, secret.Name, secret.Type, updateErr),
			Status:       0,
		})
		return "", fmt.Errorf("更新 Secret 失败")
	}

	// 记录成功的审计日志
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		WorkspaceId:  req.WorkloadId,
		Title:        "更新 Secret",
		ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 成功更新 Secret %s, 类型: %s, 更新后包含 %d 个数据项", username, secret.Namespace, secret.Name, secret.Type, len(secret.Data)),
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("成功更新 Secret: %s", secret.Name)
	return "更新成功", nil
}
