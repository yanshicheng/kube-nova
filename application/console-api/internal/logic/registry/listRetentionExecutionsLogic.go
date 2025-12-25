package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListRetentionExecutionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 列出保留策略执行历史
func NewListRetentionExecutionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRetentionExecutionsLogic {
	return &ListRetentionExecutionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListRetentionExecutionsLogic) ListRetentionExecutions(req *types.ListRetentionExecutionsRequest) (resp *types.ListRetentionExecutionsResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.ListRetentionExecutions(l.ctx, &pb.ListRetentionExecutionsReq{
		RegistryUuid: req.RegistryUuid,
		PolicyId:     req.PolicyId,
		Page:         req.Page,
		PageSize:     req.PageSize,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	var items []types.RetentionExecution
	for _, item := range rpcResp.Items {
		items = append(items, types.RetentionExecution{
			Id:        item.Id,
			PolicyId:  item.PolicyId,
			Status:    item.Status,
			Trigger:   item.Trigger,
			StartTime: item.StartTime,
			EndTime:   item.EndTime,
			DryRun:    item.DryRun,
		})
	}

	return &types.ListRetentionExecutionsResponse{
		Items:      items,
		Page:       rpcResp.Page,
		PageSize:   rpcResp.PageSize,
		Total:      rpcResp.Total,
		TotalPages: rpcResp.TotalPages,
	}, nil
}
