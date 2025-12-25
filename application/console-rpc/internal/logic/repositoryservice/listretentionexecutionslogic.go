package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRetentionExecutionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRetentionExecutionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRetentionExecutionsLogic {
	return &ListRetentionExecutionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListRetentionExecutionsLogic) ListRetentionExecutions(in *pb.ListRetentionExecutionsReq) (*pb.ListRetentionExecutionsResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	req := types.ListRequest{
		Page:     in.Page,
		PageSize: in.PageSize,
	}

	resp, err := client.Retention().ListExecutions(in.PolicyId, req)
	if err != nil {
		return nil, errorx.Msg("查询保留策略执行历史失败")
	}

	var items []*pb.RetentionExecution
	for _, exec := range resp.Items {
		dryRun := int32(0)
		if exec.DryRun {
			dryRun = 1
		}

		items = append(items, &pb.RetentionExecution{
			Id:        exec.ID,
			PolicyId:  exec.PolicyID,
			Status:    exec.Status,
			Trigger:   exec.Trigger,
			StartTime: exec.StartTime.Unix(),
			EndTime:   exec.EndTime.Unix(),
			DryRun:    dryRun,
		})
	}

	return &pb.ListRetentionExecutionsResp{
		Items:      items,
		Total:      int64(resp.Total),
		Page:       in.Page,
		PageSize:   in.PageSize,
		TotalPages: int64(resp.TotalPages),
	}, nil
}
