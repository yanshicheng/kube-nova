package project

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type SearchProjectWorkspaceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewSearchProjectWorkspaceLogic 搜索项目的工作空间列表，projectClusterId为必传参数
func NewSearchProjectWorkspaceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchProjectWorkspaceLogic {
	return &SearchProjectWorkspaceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchProjectWorkspaceLogic) SearchProjectWorkspace(req *types.SearchProjectWorkspaceRequest) (resp []types.ProjectWorkspace, err error) {

	// 调用RPC服务搜索项目工作空间
	rpcResp, err := l.svcCtx.ManagerRpc.ProjectWorkspaceSearch(l.ctx, &pb.SearchOnecProjectWorkspaceReq{
		ProjectClusterId: req.ProjectClusterId,
		Name:             req.Name,
		Namespace:        req.Namespace,
	})

	if err != nil {
		l.Errorf("搜索项目工作空间失败: %v", err)
		return nil, fmt.Errorf("搜索项目工作空间失败: %v", err)
	}

	// 转换响应数据
	workspaces := make([]types.ProjectWorkspace, 0, len(rpcResp.Data))
	for _, w := range rpcResp.Data {
		workspaces = append(workspaces, types.ProjectWorkspace{
			Id:                                      w.Id,
			ProjectClusterId:                        w.ProjectClusterId,
			ClusterUuid:                             w.ClusterUuid,
			ClusterName:                             w.ClusterName,
			Name:                                    w.Name,
			Namespace:                               w.Namespace,
			Description:                             w.Description,
			CpuAllocated:                            w.CpuAllocated,
			MemAllocated:                            w.MemAllocated,
			StorageAllocated:                        w.StorageAllocated,
			GpuAllocated:                            w.GpuAllocated,
			PodsAllocated:                           w.PodsAllocated,
			ConfigmapAllocated:                      w.ConfigmapAllocated,
			SecretAllocated:                         w.SecretAllocated,
			PvcAllocated:                            w.PvcAllocated,
			EphemeralStorageAllocated:               w.EphemeralStorageAllocated,
			ServiceAllocated:                        w.ServiceAllocated,
			LoadbalancersAllocated:                  w.LoadbalancersAllocated,
			NodeportsAllocated:                      w.NodeportsAllocated,
			DeploymentsAllocated:                    w.DeploymentsAllocated,
			JobsAllocated:                           w.JobsAllocated,
			CronjobsAllocated:                       w.CronjobsAllocated,
			DaemonsetsAllocated:                     w.DaemonsetsAllocated,
			StatefulsetsAllocated:                   w.StatefulsetsAllocated,
			IngressesAllocated:                      w.IngressesAllocated,
			PodMaxCpu:                               w.PodMaxCpu,
			PodMaxMemory:                            w.PodMaxMemory,
			PodMaxEphemeralStorage:                  w.PodMaxEphemeralStorage,
			PodMinCpu:                               w.PodMinCpu,
			PodMinMemory:                            w.PodMinMemory,
			PodMinEphemeralStorage:                  w.PodMinEphemeralStorage,
			ContainerMaxCpu:                         w.ContainerMaxCpu,
			ContainerMaxMemory:                      w.ContainerMaxMemory,
			ContainerMaxEphemeralStorage:            w.ContainerMaxEphemeralStorage,
			ContainerMinCpu:                         w.ContainerMinCpu,
			ContainerMinMemory:                      w.ContainerMinMemory,
			ContainerMinEphemeralStorage:            w.ContainerMinEphemeralStorage,
			ContainerDefaultCpu:                     w.ContainerDefaultCpu,
			ContainerDefaultMemory:                  w.ContainerDefaultMemory,
			ContainerDefaultEphemeralStorage:        w.ContainerDefaultEphemeralStorage,
			ContainerDefaultRequestCpu:              w.ContainerDefaultRequestCpu,
			ContainerDefaultRequestMemory:           w.ContainerDefaultRequestMemory,
			ContainerDefaultRequestEphemeralStorage: w.ContainerDefaultRequestEphemeralStorage,
			IsSystem:                                w.IsSystem,
			Status:                                  w.Status,
			AppCreateTime:                           w.AppCreateTime,
			CreatedBy:                               w.CreatedBy,
			UpdatedBy:                               w.UpdatedBy,
			CreatedAt:                               w.CreatedAt,
			UpdatedAt:                               w.UpdatedAt,
		})
	}

	return workspaces, nil
}
