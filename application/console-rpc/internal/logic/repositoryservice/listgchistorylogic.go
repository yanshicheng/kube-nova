package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListGCHistoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListGCHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListGCHistoryLogic {
	return &ListGCHistoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ============ 垃圾回收管理 ============
func (l *ListGCHistoryLogic) ListGCHistory(in *pb.ListGCHistoryReq) (*pb.ListGCHistoryResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	req := types.ListRequest{
		Page:     in.Page,
		PageSize: in.PageSize,
	}

	resp, err := client.GC().ListHistory(req)
	if err != nil {
		return nil, errorx.Msg("查询GC历史失败")
	}

	var items []*pb.GCHistory
	for _, gc := range resp.Items {
		item := &pb.GCHistory{
			Id:             gc.ID,
			JobName:        gc.JobName,
			JobKind:        gc.JobKind,
			JobStatus:      gc.JobStatus,
			CreationTime:   gc.CreationTime.Unix(),
			UpdateTime:     gc.UpdateTime.Unix(),
			DeleteUntagged: gc.DeleteUntagged,
			JobParameters:  gc.JobParameters,
		}

		items = append(items, item)
	}

	return &pb.ListGCHistoryResp{
		Items:      items,
		Total:      int64(resp.Total),
		Page:       in.Page,
		PageSize:   in.PageSize,
		TotalPages: int64(resp.TotalPages),
	}, nil
}
