package pod

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type PodGetDefaultContainerLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Pod 默认容器
func NewPodGetDefaultContainerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PodGetDefaultContainerLogic {
	return &PodGetDefaultContainerLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PodGetDefaultContainerLogic) PodGetDefaultContainer(req *types.PodGetDefaultContainerReq) (resp *types.PodGetDefaultContainerResp, err error) {
	// 1. 获取集群和命名空间信息
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return nil, fmt.Errorf("获取项目工作空间详情失败")
	}

	// 2. 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 3. 初始化 Pod 操作器
	podClient := client.Pods()

	// 4. 获取默认容器名称
	// 默认容器的选择规则：
	// 1) 如果 Pod 有注解 kubectl.kubernetes.io/default-container，使用该容器
	// 2) 否则使用第一个非 Init 容器
	containerName, err := podClient.GetDefaultContainer(workloadInfo.Data.Namespace, req.PodName)
	if err != nil {
		l.Errorf("获取默认容器失败: %v", err)
		return nil, fmt.Errorf("获取默认容器失败: %v", err)
	}

	resp = &types.PodGetDefaultContainerResp{
		ContainerName: containerName,
	}

	l.Infof("成功获取 Pod [%s] 的默认容器: %s", req.PodName, containerName)

	return resp, nil
}
