package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListGCHistoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 列出GC历史
func NewListGCHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListGCHistoryLogic {
	return &ListGCHistoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListGCHistoryLogic) ListGCHistory(req *types.ListGCHistoryRequest) (resp *types.ListGCHistoryResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.ListGCHistory(l.ctx, &pb.ListGCHistoryReq{
		RegistryUuid: req.RegistryUuid,
		Page:         req.Page,
		PageSize:     req.PageSize,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	var items []types.GCHistory
	for _, item := range rpcResp.Items {
		items = append(items, types.GCHistory{
			Id:             item.Id,
			JobName:        item.JobName,
			JobKind:        item.JobKind,
			JobStatus:      item.JobStatus,
			CreationTime:   item.CreationTime,
			UpdateTime:     item.UpdateTime,
			DeleteUntagged: item.DeleteUntagged,
			JobParameters:  item.JobParameters,
		})
	}

	l.Infof("GC历史查询成功: Total=%d", rpcResp.Total)
	return &types.ListGCHistoryResponse{
		Items:      items,
		Total:      rpcResp.Total,
		Page:       rpcResp.Page,
		PageSize:   rpcResp.PageSize,
		TotalPages: rpcResp.TotalPages,
	}, nil
}
