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

type UpdateProjectClusterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewUpdateProjectClusterLogic 更新项目集群资源配额，不能小于已使用量
func NewUpdateProjectClusterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProjectClusterLogic {
	return &UpdateProjectClusterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateProjectClusterLogic) UpdateProjectCluster(req *types.UpdateProjectClusterRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 先查询原数据用于审计日志对比
	oldData, queryErr := l.svcCtx.ManagerRpc.ProjectClusterGetById(l.ctx, &pb.GetOnecProjectClusterByIdReq{
		Id: req.Id,
	})
	var clusterUuid string
	var projectId uint64
	var oldCpuLimit string
	var oldMemLimit string
	var oldStorageLimit string
	if queryErr == nil && oldData.Data != nil {
		clusterUuid = oldData.Data.ClusterUuid
		projectId = uint64(oldData.Data.ProjectId)
		oldCpuLimit = oldData.Data.CpuLimit
		oldMemLimit = oldData.Data.MemLimit
		oldStorageLimit = oldData.Data.StorageLimit
	}

	// 调用RPC服务更新项目集群配额
	_, err = l.svcCtx.ManagerRpc.ProjectClusterUpdate(l.ctx, &pb.UpdateOnecProjectClusterReq{
		Id:                    req.Id,
		CpuLimit:              req.CpuLimit,
		CpuOvercommitRatio:    req.CpuOvercommitRatio,
		CpuCapacity:           req.CpuCapacity,
		MemLimit:              req.MemLimit,
		MemOvercommitRatio:    req.MemOvercommitRatio,
		MemCapacity:           req.MemCapacity,
		StorageLimit:          req.StorageLimit,
		GpuLimit:              req.GpuLimit,
		GpuOvercommitRatio:    req.GpuOvercommitRatio,
		GpuCapacity:           req.GpuCapacity,
		PodsLimit:             req.PodsLimit,
		ConfigmapLimit:        req.ConfigmapLimit,
		SecretLimit:           req.SecretLimit,
		PvcLimit:              req.PvcLimit,
		EphemeralStorageLimit: req.EphemeralStorageLimit,
		ServiceLimit:          req.ServiceLimit,
		LoadbalancersLimit:    req.LoadbalancersLimit,
		NodeportsLimit:        req.NodeportsLimit,
		DeploymentsLimit:      req.DeploymentsLimit,
		JobsLimit:             req.JobsLimit,
		CronjobsLimit:         req.CronjobsLimit,
		DaemonsetsLimit:       req.DaemonsetsLimit,
		StatefulsetsLimit:     req.StatefulsetsLimit,
		IngressesLimit:        req.IngressesLimit,
		UpdatedBy:             username,
	})

	// 记录审计日志
	auditStatus := int64(1)
	if err != nil {
		auditStatus = 0
	}
	actionDetail := fmt.Sprintf("用户 %s 更新项目集群配额, 配额ID: %d, CPU限制: %s -> %s, 内存限制: %s -> %s, 存储限制: %s -> %s",
		username, req.Id, oldCpuLimit, req.CpuLimit, oldMemLimit, req.MemLimit, oldStorageLimit, req.StorageLimit)
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  clusterUuid,
		ProjectId:    projectId,
		Title:        "更新项目集群配额",
		ActionDetail: actionDetail,
		Status:       auditStatus,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	if err != nil {
		l.Errorf("更新项目集群配额失败: %v", err)
		return "", fmt.Errorf("更新项目集群配额失败: %v", err)
	}

	return "项目集群配额更新成功", nil
}
