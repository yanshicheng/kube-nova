package project

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateProjectWorkspaceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewUpdateProjectWorkspaceLogic 更新工作空间配置，资源配额不能超过项目集群限制
func NewUpdateProjectWorkspaceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProjectWorkspaceLogic {
	return &UpdateProjectWorkspaceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateProjectWorkspaceLogic) UpdateProjectWorkspace(req *types.UpdateProjectWorkspaceRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 先查询原数据用于审计日志对比
	oldData, queryErr := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &pb.GetOnecProjectWorkspaceByIdReq{
		Id: req.Id,
	})
	var oldName string
	var oldCpuAllocated string
	var oldMemAllocated string
	var oldStorageAllocated string
	var clusterUuid string
	if queryErr == nil && oldData.Data != nil {
		oldName = oldData.Data.Name
		oldCpuAllocated = oldData.Data.CpuAllocated
		oldMemAllocated = oldData.Data.MemAllocated
		oldStorageAllocated = oldData.Data.StorageAllocated
		clusterUuid = oldData.Data.ClusterUuid
	}

	// 调用RPC服务更新项目工作空间
	_, err = l.svcCtx.ManagerRpc.ProjectWorkspaceUpdate(l.ctx, &pb.UpdateOnecProjectWorkspaceReq{
		Id:                                      req.Id,
		Name:                                    req.Name,
		Description:                             req.Description,
		CpuAllocated:                            req.CpuAllocated,
		MemAllocated:                            req.MemAllocated,
		StorageAllocated:                        req.StorageAllocated,
		GpuAllocated:                            req.GpuAllocated,
		PodsAllocated:                           req.PodsAllocated,
		ConfigmapAllocated:                      req.ConfigmapAllocated,
		SecretAllocated:                         req.SecretAllocated,
		PvcAllocated:                            req.PvcAllocated,
		EphemeralStorageAllocated:               req.EphemeralStorageAllocated,
		ServiceAllocated:                        req.ServiceAllocated,
		LoadbalancersAllocated:                  req.LoadbalancersAllocated,
		NodeportsAllocated:                      req.NodeportsAllocated,
		DeploymentsAllocated:                    req.DeploymentsAllocated,
		JobsAllocated:                           req.JobsAllocated,
		CronjobsAllocated:                       req.CronjobsAllocated,
		DaemonsetsAllocated:                     req.DaemonsetsAllocated,
		StatefulsetsAllocated:                   req.StatefulsetsAllocated,
		IngressesAllocated:                      req.IngressesAllocated,
		PodMaxCpu:                               req.PodMaxCpu,
		PodMaxMemory:                            req.PodMaxMemory,
		PodMaxEphemeralStorage:                  req.PodMaxEphemeralStorage,
		PodMinCpu:                               req.PodMinCpu,
		PodMinMemory:                            req.PodMinMemory,
		PodMinEphemeralStorage:                  req.PodMinEphemeralStorage,
		ContainerMaxCpu:                         req.ContainerMaxCpu,
		ContainerMaxMemory:                      req.ContainerMaxMemory,
		ContainerMaxEphemeralStorage:            req.ContainerMaxEphemeralStorage,
		ContainerMinCpu:                         req.ContainerMinCpu,
		ContainerMinMemory:                      req.ContainerMinMemory,
		ContainerMinEphemeralStorage:            req.ContainerMinEphemeralStorage,
		ContainerDefaultCpu:                     req.ContainerDefaultCpu,
		ContainerDefaultMemory:                  req.ContainerDefaultMemory,
		ContainerDefaultEphemeralStorage:        req.ContainerDefaultEphemeralStorage,
		ContainerDefaultRequestCpu:              req.ContainerDefaultRequestCpu,
		ContainerDefaultRequestMemory:           req.ContainerDefaultRequestMemory,
		ContainerDefaultRequestEphemeralStorage: req.ContainerDefaultRequestEphemeralStorage,
		UpdatedBy:                               username,
	})

	// 记录审计日志
	auditStatus := int64(1)
	if err != nil {
		auditStatus = 0
	}
	actionDetail := fmt.Sprintf("用户 %s 更新项目工作空间, 工作空间ID: %d, 名称: %s -> %s, CPU分配: %s -> %s, 内存分配: %s -> %s, 存储分配: %s -> %s",
		username, req.Id, oldName, req.Name, oldCpuAllocated, req.CpuAllocated, oldMemAllocated, req.MemAllocated, oldStorageAllocated, req.StorageAllocated)
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  clusterUuid,
		WorkspaceId:  uint64(req.Id),
		Title:        "更新项目工作空间",
		ActionDetail: actionDetail,
		Status:       auditStatus,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	if err != nil {
		l.Errorf("更新项目工作空间失败: %v", err)
		return "", fmt.Errorf("更新项目工作空间失败: %v", err)
	}

	return "项目工作空间更新成功", nil
}
