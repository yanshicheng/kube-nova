package alertservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type UnbindAlertGroupAppsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUnbindAlertGroupAppsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnbindAlertGroupAppsLogic {
	return &UnbindAlertGroupAppsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UnbindAlertGroupApps 取消关联app
func (l *UnbindAlertGroupAppsLogic) UnbindAlertGroupApps(in *pb.UnbindAlertGroupAppsReq) (*pb.UnbindAlertGroupAppsResp, error) {
	// 1. 检查关联是否存在
	data, err := l.svcCtx.AlertGroupAppsModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("告警组应用关联不存在: id=%d, err=%v", in.Id, err)
		return nil, errorx.Msg("告警组应用关联不存在")
	}

	// 2. 软删除关联
	err = l.svcCtx.AlertGroupAppsModel.DeleteSoft(l.ctx, in.Id)
	if err != nil {
		l.Errorf("删除告警组应用关联失败: id=%d, err=%v", in.Id, err)
		return nil, errorx.Msg("删除告警组应用关联失败")
	}

	l.Infof("成功删除告警组应用关联: id=%d, groupId=%d, appId=%d, appType=%s",
		in.Id, data.GroupId, data.AppId, data.AppType)
	return &pb.UnbindAlertGroupAppsResp{}, nil
}
