package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAlertChannelsByIdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetAlertChannelsByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertChannelsByIdLogic {
	return &GetAlertChannelsByIdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAlertChannelsByIdLogic) GetAlertChannelsById(req *types.DefaultIdRequest) (resp *types.AlertChannels, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("获取告警渠道请求: operator=%s, id=%d", username, req.Id)

	// 调用 RPC 服务获取告警渠道
	result, err := l.svcCtx.AlertPortalRpc.AlertChannelsGetById(l.ctx, &pb.GetAlertChannelsByIdReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取告警渠道失败: operator=%s, id=%d, error=%v", username, req.Id, err)
		return nil, err
	}

	// 转换数据
	resp = &types.AlertChannels{
		Id:          result.Data.Id,
		Uuid:        result.Data.Uuid,
		ChannelName: result.Data.ChannelName,
		ChannelType: result.Data.ChannelType,
		//Config:      result.Data.Config,
		Description: result.Data.Description,
		RetryTimes:  result.Data.RetryTimes,
		Timeout:     result.Data.Timeout,
		RateLimit:   result.Data.RateLimit,
		CreatedBy:   result.Data.CreatedBy,
		UpdatedBy:   result.Data.UpdatedBy,
		CreatedAt:   result.Data.CreatedAt,
		UpdatedAt:   result.Data.UpdatedAt,
	}

	l.Infof("获取告警渠道成功: operator=%s, id=%d", username, req.Id)
	return resp, nil
}
