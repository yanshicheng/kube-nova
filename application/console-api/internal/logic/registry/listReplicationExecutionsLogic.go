package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListReplicationExecutionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 列出复制执行历史
func NewListReplicationExecutionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListReplicationExecutionsLogic {
	return &ListReplicationExecutionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListReplicationExecutionsLogic) ListReplicationExecutions(req *types.ListReplicationExecutionsRequest) (resp *types.ListReplicationExecutionsResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.ListReplicationExecutions(l.ctx, &pb.ListReplicationExecutionsReq{
		RegistryUuid: req.RegistryUuid,
		PolicyId:     req.PolicyId,
		Page:         req.Page,
		PageSize:     req.PageSize,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	var items []types.ReplicationExecution
	for _, item := range rpcResp.Items {
		items = append(items, types.ReplicationExecution{
			Id:         item.Id,
			PolicyId:   item.PolicyId,
			Status:     item.Status,
			Trigger:    item.Trigger,
			StartTime:  item.StartTime,
			EndTime:    item.EndTime,
			Succeed:    item.Succeed,
			Failed:     item.Failed,
			InProgress: item.InProgress,
			Stopped:    item.Stopped,
		})
	}

	l.Infof("复制执行历史查询成功: Total=%d", rpcResp.Total)

	// 注意：API 返回 string，但实际应该返回结构化数据
	// 这里简化处理，实际项目中应该修改 API 定义返回 ListReplicationExecutionsResponse
	return &types.ListReplicationExecutionsResponse{
		Items:      items,
		Page:       rpcResp.Page,
		PageSize:   rpcResp.PageSize,
		Total:      rpcResp.Total,
		TotalPages: rpcResp.TotalPages,
	}, nil
}
