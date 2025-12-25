package node

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetNodeDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeDetailLogic {
	return &GetNodeDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeDetailLogic) GetNodeDetail(req *types.NodeIdRequest) (resp *types.ClusterNodeDetail, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.NodeDetail(l.ctx, &pb.ClusterNodeDetailReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取节点详情失败: %v", err)
		return nil, errorx.Msg("获取节点详情失败")
	}

	return &types.ClusterNodeDetail{
		Id:              rpcResp.Id,
		ClusterUuid:     rpcResp.ClusterUuid,
		NodeUuid:        rpcResp.NodeUuid,
		Name:            rpcResp.Name,
		Hostname:        rpcResp.Hostname,
		Roles:           rpcResp.Roles,
		OsImage:         rpcResp.OsImage,
		NodeIp:          rpcResp.NodeIp,
		KernelVersion:   rpcResp.KernelVersion,
		OperatingSystem: rpcResp.OperatingSystem,
		Architecture:    rpcResp.Architecture,
		Cpu:             rpcResp.Cpu,
		Memory:          rpcResp.Memory,
		Pods:            rpcResp.Pods,
		IsGpu:           rpcResp.IsGpu,
		Runtime:         rpcResp.Runtime,
		JoinAt:          rpcResp.JoinAt,
		Unschedulable:   rpcResp.Unschedulable,
		KubeletVersion:  rpcResp.KubeletVersion,
		Status:          rpcResp.Status,
		PodCidr:         rpcResp.PodCidr,
		PodCidrs:        rpcResp.PodCidrs,
		CreatedBy:       rpcResp.CreatedBy,
		UpdatedBy:       rpcResp.UpdatedBy,
		CreatedAt:       rpcResp.CreatedAt,
		UpdatedAt:       rpcResp.UpdatedAt,
	}, nil
}
