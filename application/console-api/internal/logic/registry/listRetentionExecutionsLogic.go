package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
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
	if req.PolicyId <= 0 {
		l.Errorf("查询执行历史失败: PolicyId 无效 (policyId=%d)", req.PolicyId)
		return nil, errorx.Msg("策略ID无效，请先配置保留策略")
	}

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

	// 处理空结果
	var items []types.RetentionExecution
	if rpcResp.Items != nil {
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
	}

	// 确保 items 不为 nil，返回空数组而非 null
	if items == nil {
		items = []types.RetentionExecution{}
	}

	l.Infof("查询执行历史成功: policyId=%d, total=%d", req.PolicyId, rpcResp.Total)
	return &types.ListRetentionExecutionsResponse{
		Items:      items,
		Page:       rpcResp.Page,
		PageSize:   rpcResp.PageSize,
		Total:      rpcResp.Total,
		TotalPages: rpcResp.TotalPages,
	}, nil
}
