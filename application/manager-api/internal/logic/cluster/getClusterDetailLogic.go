package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetClusterDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterDetailLogic {
	return &GetClusterDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterDetailLogic) GetClusterDetail(req *types.DefaultIdRequest) (resp *types.ClusterDetail, err error) {

	// 调用RPC获取集群详情
	rpcResp, err := l.svcCtx.ManagerRpc.ClusterDetail(l.ctx, &pb.ClusterDetailReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("RPC调用获取集群详情失败: %v", err)
		return nil, fmt.Errorf("获取集群详情失败: %w", err)
	}

	// 转换响应
	resp = &types.ClusterDetail{
		Id:               rpcResp.Id,
		Name:             rpcResp.Name,
		Avatar:           rpcResp.Avatar,
		Uuid:             rpcResp.Uuid,
		Description:      rpcResp.Description,
		ClusterType:      rpcResp.ClusterType,
		Environment:      rpcResp.Environment,
		Region:           rpcResp.Region,
		Zone:             rpcResp.Zone,
		Datacenter:       rpcResp.Datacenter,
		Provider:         rpcResp.Provider,
		IsManaged:        rpcResp.IsManaged,
		NodeLb:           rpcResp.NodeLb,
		MasterLb:         rpcResp.MasterLb,
		IngressDomain:    rpcResp.IngressDomain,
		Status:           rpcResp.Status,
		HealthStatus:     rpcResp.HealthStatus,
		LastSyncAt:       rpcResp.LastSyncAt,
		Version:          rpcResp.Version,
		GitCommit:        rpcResp.GitCommit,
		Platform:         rpcResp.Platform,
		VersionBuildAt:   rpcResp.VersionBuildAt,
		ClusterCreatedAt: rpcResp.ClusterCreatedAt,
		CostCenter:       rpcResp.CostCenter,
		BusinessUnit:     rpcResp.BusinessUnit,
		OwnerTeam:        rpcResp.OwnerTeam,
		OwnerEmail:       rpcResp.OwnerEmail,
		Priority:         rpcResp.Priority,
		CreatedBy:        rpcResp.CreatedBy,
		UpdatedBy:        rpcResp.UpdatedBy,
		CreatedAt:        rpcResp.CreatedAt,
		UpdatedAt:        rpcResp.UpdatedAt,
	}

	return resp, nil
}
