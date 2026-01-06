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

type AddProjectClusterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewAddProjectClusterLogic 为项目分配集群资源配额
func NewAddProjectClusterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddProjectClusterLogic {
	return &AddProjectClusterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddProjectClusterLogic) AddProjectCluster(req *types.AddProjectClusterRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用RPC服务添加项目集群配额
	_, err = l.svcCtx.ManagerRpc.ProjectClusterAdd(l.ctx, &pb.AddOnecProjectClusterReq{
		ClusterUuid:           req.ClusterUuid,
		ProjectId:             req.ProjectId,
		PriceConfigId:         req.PriceConfigId,
		BillingStartTime:      req.BillingStartTime,
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
		CreatedBy:             username,
		UpdatedBy:             username,
	})

	// 记录审计日志
	auditStatus := int64(1)
	if err != nil {
		auditStatus = 0
	}
	actionDetail := fmt.Sprintf("用户 %s 为项目分配集群资源配额, 集群UUID: %s, CPU限制: %s, 内存限制: %s, 存储限制: %s, GPU限制: %s, Pod限制: %d",
		username, req.ClusterUuid, req.CpuLimit, req.MemLimit, req.StorageLimit, req.GpuLimit, req.PodsLimit)
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ClusterUuid:  req.ClusterUuid,
		ProjectId:    uint64(req.ProjectId),
		Title:        "添加项目集群配额",
		ActionDetail: actionDetail,
		Status:       auditStatus,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	if err != nil {
		l.Errorf("添加项目集群配额失败: %v", err)
		return "", fmt.Errorf("添加项目集群配额失败: %v", err)
	}

	return "项目集群配额添加成功", nil
}
