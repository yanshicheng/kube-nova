package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetServiceYamlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Service YAML
func NewGetServiceYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetServiceYamlLogic {
	return &GetServiceYamlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetServiceYamlLogic) GetServiceYaml(req *types.DefaultNameRequest) (resp string, err error) {
	// 1. 获取集群以及命名空间
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return "", fmt.Errorf("获取项目工作空间详情失败")
	}

	// 2. 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 3. 初始化 Service 客户端
	serviceClient := client.Services()

	// 4. 获取 Service YAML
	yamlStr, err := serviceClient.GetYaml(workloadInfo.Data.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 Service YAML 失败: %v", err)
		return "", fmt.Errorf("获取 Service YAML 失败")
	}

	l.Infof("成功获取 Service YAML: %s", req.Name)
	return yamlStr, nil
}
