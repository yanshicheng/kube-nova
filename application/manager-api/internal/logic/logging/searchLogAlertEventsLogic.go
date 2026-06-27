// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logging

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	logservice "github.com/yanshicheng/kube-nova/application/manager-rpc/client/logservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchLogAlertEventsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询日志告警事件
func NewSearchLogAlertEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchLogAlertEventsLogic {
	return &SearchLogAlertEventsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchLogAlertEventsLogic) SearchLogAlertEvents(req *types.SearchLogAlertEventsRequest) (resp *types.SearchLogAlertEventsResponse, err error) {
	rpcResp, err := l.svcCtx.LogRpc.SearchLogAlertEvents(l.ctx, &logservice.SearchLogAlertEventsReq{
		Page:         req.Page,
		PageSize:     req.PageSize,
		ClusterUuid:  req.ClusterUuid,
		RuleId:       req.RuleId,
		EventStatus:  req.EventStatus,
		NotifyStatus: req.NotifyStatus,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.LogAlertEvent, 0, len(rpcResp.Data))
	for _, item := range rpcResp.Data {
		items = append(items, convertLogAlertEvent(item))
	}
	return &types.SearchLogAlertEventsResponse{
		Data:  items,
		Total: rpcResp.Total,
	}, nil
}
