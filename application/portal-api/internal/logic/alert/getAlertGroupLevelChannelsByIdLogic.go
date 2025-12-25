package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAlertGroupLevelChannelsByIdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetAlertGroupLevelChannelsByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertGroupLevelChannelsByIdLogic {
	return &GetAlertGroupLevelChannelsByIdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAlertGroupLevelChannelsByIdLogic) GetAlertGroupLevelChannelsById(req *types.DefaultIdRequest) (resp *types.AlertGroupLevelChannels, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("获取告警组级别渠道请求: operator=%s, id=%d", username, req.Id)

	// 调用 RPC 服务获取告警组级别渠道
	result, err := l.svcCtx.AlertPortalRpc.AlertGroupLevelChannelsGetById(l.ctx, &pb.GetAlertGroupLevelChannelsByIdReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取告警组级别渠道失败: operator=%s, id=%d, error=%v", username, req.Id, err)
		return nil, err
	}

	// 转换数据
	resp = &types.AlertGroupLevelChannels{
		Id:        result.Data.Id,
		GroupId:   result.Data.GroupId,
		Severity:  result.Data.Severity,
		ChannelId: result.Data.ChannelId,
		CreatedBy: result.Data.CreatedBy,
		UpdatedBy: result.Data.UpdatedBy,
		CreatedAt: result.Data.CreatedAt,
		UpdatedAt: result.Data.UpdatedAt,
	}

	l.Infof("获取告警组级别渠道成功: operator=%s, id=%d", username, req.Id)
	return resp, nil
}
