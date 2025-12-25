package project

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetProjectWorkspaceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewGetProjectWorkspaceLogic 根据ID获取工作空间详细信息
func NewGetProjectWorkspaceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectWorkspaceLogic {
	return &GetProjectWorkspaceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProjectWorkspaceLogic) GetProjectWorkspace(req *types.DefaultIdRequest) (resp *types.ProjectWorkspace, err error) {

	// 调用RPC服务获取项目工作空间详情
	rpcResp, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &pb.GetOnecProjectWorkspaceByIdReq{
		Id: req.Id,
	})

	if err != nil {
		l.Errorf("查询项目工作空间详情失败: %v", err)
		return nil, fmt.Errorf("查询项目工作空间详情失败: %v", err)
	}

	if rpcResp.Data == nil {
		l.Errorf("项目工作空间不存在, ID: %d", req.Id)
		return nil, fmt.Errorf("项目工作空间不存在")
	}

	// 转换响应数据
	workspace := &types.ProjectWorkspace{
		Id:                                      rpcResp.Data.Id,
		ProjectClusterId:                        rpcResp.Data.ProjectClusterId,
		ClusterUuid:                             rpcResp.Data.ClusterUuid,
		ClusterName:                             rpcResp.Data.ClusterName,
		Name:                                    rpcResp.Data.Name,
		Namespace:                               rpcResp.Data.Namespace,
		Description:                             rpcResp.Data.Description,
		CpuAllocated:                            rpcResp.Data.CpuAllocated,
		MemAllocated:                            rpcResp.Data.MemAllocated,
		StorageAllocated:                        rpcResp.Data.StorageAllocated,
		GpuAllocated:                            rpcResp.Data.GpuAllocated,
		PodsAllocated:                           rpcResp.Data.PodsAllocated,
		ConfigmapAllocated:                      rpcResp.Data.ConfigmapAllocated,
		SecretAllocated:                         rpcResp.Data.SecretAllocated,
		PvcAllocated:                            rpcResp.Data.PvcAllocated,
		EphemeralStorageAllocated:               rpcResp.Data.EphemeralStorageAllocated,
		ServiceAllocated:                        rpcResp.Data.ServiceAllocated,
		LoadbalancersAllocated:                  rpcResp.Data.LoadbalancersAllocated,
		NodeportsAllocated:                      rpcResp.Data.NodeportsAllocated,
		DeploymentsAllocated:                    rpcResp.Data.DeploymentsAllocated,
		JobsAllocated:                           rpcResp.Data.JobsAllocated,
		CronjobsAllocated:                       rpcResp.Data.CronjobsAllocated,
		DaemonsetsAllocated:                     rpcResp.Data.DaemonsetsAllocated,
		StatefulsetsAllocated:                   rpcResp.Data.StatefulsetsAllocated,
		IngressesAllocated:                      rpcResp.Data.IngressesAllocated,
		PodMaxCpu:                               rpcResp.Data.PodMaxCpu,
		PodMaxMemory:                            rpcResp.Data.PodMaxMemory,
		PodMaxEphemeralStorage:                  rpcResp.Data.PodMaxEphemeralStorage,
		PodMinCpu:                               rpcResp.Data.PodMinCpu,
		PodMinMemory:                            rpcResp.Data.PodMinMemory,
		PodMinEphemeralStorage:                  rpcResp.Data.PodMinEphemeralStorage,
		ContainerMaxCpu:                         rpcResp.Data.ContainerMaxCpu,
		ContainerMaxMemory:                      rpcResp.Data.ContainerMaxMemory,
		ContainerMaxEphemeralStorage:            rpcResp.Data.ContainerMaxEphemeralStorage,
		ContainerMinCpu:                         rpcResp.Data.ContainerMinCpu,
		ContainerMinMemory:                      rpcResp.Data.ContainerMinMemory,
		ContainerMinEphemeralStorage:            rpcResp.Data.ContainerMinEphemeralStorage,
		ContainerDefaultCpu:                     rpcResp.Data.ContainerDefaultCpu,
		ContainerDefaultMemory:                  rpcResp.Data.ContainerDefaultMemory,
		ContainerDefaultEphemeralStorage:        rpcResp.Data.ContainerDefaultEphemeralStorage,
		ContainerDefaultRequestCpu:              rpcResp.Data.ContainerDefaultRequestCpu,
		ContainerDefaultRequestMemory:           rpcResp.Data.ContainerDefaultRequestMemory,
		ContainerDefaultRequestEphemeralStorage: rpcResp.Data.ContainerDefaultRequestEphemeralStorage,
		IsSystem:                                rpcResp.Data.IsSystem,
		Status:                                  rpcResp.Data.Status,
		AppCreateTime:                           rpcResp.Data.AppCreateTime,
		CreatedBy:                               rpcResp.Data.CreatedBy,
		UpdatedBy:                               rpcResp.Data.UpdatedBy,
		CreatedAt:                               rpcResp.Data.CreatedAt,
		UpdatedAt:                               rpcResp.Data.UpdatedAt,
	}

	return workspace, nil
}
