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

type AddProjectWorkspaceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewAddProjectWorkspaceLogic 创建新的项目工作空间（Kubernetes命名空间）
func NewAddProjectWorkspaceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddProjectWorkspaceLogic {
	return &AddProjectWorkspaceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddProjectWorkspaceLogic) AddProjectWorkspace(req *types.AddProjectWorkspaceRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用RPC服务创建项目工作空间
	rpcResp, err := l.svcCtx.ManagerRpc.ProjectWorkspaceAdd(l.ctx, &pb.AddOnecProjectWorkspaceReq{
		ProjectClusterId:                        req.ProjectClusterId,
		ClusterUuid:                             req.ClusterUuid,
		Name:                                    req.Name,
		Namespace:                               req.Namespace,
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
		CreatedBy:                               username,
		UpdatedBy:                               username,
	})

	// 记录审计日志
	auditStatus := int64(1)
	if err != nil {
		auditStatus = 0
	}
	var workspaceId uint64
	if rpcResp != nil {
		workspaceId = rpcResp.Id
	}
	actionDetail := fmt.Sprintf("用户 %s 创建项目工作空间, 名称: %s, 命名空间: %s, 集群UUID: %s, CPU分配: %d, 内存分配: %d, 存储分配: %d",
		username, req.Name, req.Namespace, req.ClusterUuid, req.CpuAllocated, req.MemAllocated, req.StorageAllocated)
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		WorkspaceId:  workspaceId,
		Title:        "创建项目工作空间",
		ActionDetail: actionDetail,
		Status:       auditStatus,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	if err != nil {
		l.Errorf("创建项目工作空间失败: %v", err)
		return "", fmt.Errorf("创建项目工作空间失败: %v", err)
	}

	return "项目工作空间创建成功", nil
}
