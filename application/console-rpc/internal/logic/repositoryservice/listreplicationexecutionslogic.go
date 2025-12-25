package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListReplicationExecutionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListReplicationExecutionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListReplicationExecutionsLogic {
	return &ListReplicationExecutionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListReplicationExecutionsLogic) ListReplicationExecutions(in *pb.ListReplicationExecutionsReq) (*pb.ListReplicationExecutionsResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	req := types.ListRequest{
		Page:     in.Page,
		PageSize: in.PageSize,
	}

	resp, err := client.Replication().ListExecutions(in.PolicyId, req)
	if err != nil {
		return nil, errorx.Msg("查询复制执行历史失败")
	}

	var items []*pb.ReplicationExecution
	for _, exec := range resp.Items {
		items = append(items, &pb.ReplicationExecution{
			Id:         exec.ID,
			PolicyId:   exec.PolicyID,
			Status:     exec.Status,
			Trigger:    exec.Trigger,
			StartTime:  exec.StartTime.Unix(),
			EndTime:    exec.EndTime.Unix(),
			Succeed:    exec.Succeed,
			Failed:     exec.Failed,
			InProgress: exec.InProgress,
			Stopped:    exec.Stopped,
		})
	}

	return &pb.ListReplicationExecutionsResp{
		Items:      items,
		Total:      int64(resp.Total),
		Page:       in.Page,
		PageSize:   in.PageSize,
		TotalPages: int64(resp.TotalPages),
	}, nil
}
