package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"
)

type ServiceAccountDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceAccountDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceAccountDeleteLogic {
	return &ServiceAccountDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceAccountDeleteLogic) ServiceAccountDelete(req *types.ClusterNamespaceResourceRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 获取 ServiceAccount operator
	saOp := client.ServiceAccounts()

	// 获取 ServiceAccount 详情用于审计
	existingSA, _ := saOp.Get(req.Namespace, req.Name)
	var secretsInfo string
	var imagePullSecretsInfo string
	if existingSA != nil {
		secretsInfo = l.buildSecretsInfo(existingSA.Secrets)
		imagePullSecretsInfo = l.buildImagePullSecretsInfo(existingSA.ImagePullSecrets)
	}

	// 删除 ServiceAccount
	deleteErr := saOp.Delete(req.Namespace, req.Name)
	if deleteErr != nil {
		l.Errorf("删除 ServiceAccount 失败: %v", deleteErr)
		// 记录失败的审计日志
		_, _ = l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
			ClusterUuid:  req.ClusterUuid,
			Title:        "删除 ServiceAccount",
			ActionDetail: fmt.Sprintf("用户 %s 在命名空间 %s 删除 ServiceAccount %s 失败, 错误原因: %v", username, req.Namespace, req.Name, deleteErr),
			Status:       0,
		})
		return "", fmt.Errorf("删除 ServiceAccount 失败")
	}

	// 记录成功的审计日志
	auditDetail := fmt.Sprintf("用户 %s 在命名空间 %s 成功删除 ServiceAccount %s, Secrets: %s, ImagePullSecrets: %s", username, req.Namespace, req.Name, secretsInfo, imagePullSecretsInfo)

	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		Title:        "删除 ServiceAccount",
		ActionDetail: auditDetail,
		Status:       1,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	l.Infof("用户: %s, 成功删除 ServiceAccount: %s/%s", username, req.Namespace, req.Name)
	return "删除 ServiceAccount 成功", nil
}

// buildSecretsInfo 构建 Secrets 信息
func (l *ServiceAccountDeleteLogic) buildSecretsInfo(secrets []corev1.ObjectReference) string {
	if len(secrets) == 0 {
		return "无"
	}
	var names []string
	for _, s := range secrets {
		names = append(names, s.Name)
	}
	return strings.Join(names, ", ")
}

// buildImagePullSecretsInfo 构建 ImagePullSecrets 信息
func (l *ServiceAccountDeleteLogic) buildImagePullSecretsInfo(secrets []corev1.LocalObjectReference) string {
	if len(secrets) == 0 {
		return "无"
	}
	var names []string
	for _, s := range secrets {
		names = append(names, s.Name)
	}
	return strings.Join(names, ", ")
}
