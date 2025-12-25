package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type SecretGetYamlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Secret YAML
func NewSecretGetYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SecretGetYamlLogic {
	return &SecretGetYamlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SecretGetYamlLogic) SecretGetYaml(req *types.DefaultNameRequest) (resp string, err error) {
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

	yamlStr, err := secretClient.GetYaml(workloadInfo.Data.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 Secret YAML 失败: %v", err)
		return "", fmt.Errorf("获取 Secret YAML 失败")
	}

	l.Infof("成功获取 Secret YAML: %s", req.Name)
	return yamlStr, nil
}
