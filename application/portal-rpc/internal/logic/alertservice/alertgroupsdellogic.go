package alertservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertGroupsDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertGroupsDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertGroupsDelLogic {
	return &AlertGroupsDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlertGroupsDelLogic) AlertGroupsDel(in *pb.DelAlertGroupsReq) (*pb.DelAlertGroupsResp, error) {
	// 使用Model的Delete方法，会自动处理缓存
	// 按照外键依赖顺序删除：成员 -> 级别渠道 -> 应用关联 -> 告警组

	// 1. 查询所有子记录
	members, _ := l.svcCtx.AlertGroupMembersModel.SearchNoPage(l.ctx, "", true, "`group_id` = ?", in.Id)
	channels, _ := l.svcCtx.AlertGroupLevelChannelsModel.SearchNoPage(l.ctx, "", true, "`group_id` = ?", in.Id)
	apps, _ := l.svcCtx.AlertGroupAppsModel.SearchNoPage(l.ctx, "", true, "`group_id` = ?", in.Id)

	// 2. 删除所有成员（Delete方法会自动处理缓存）
	for _, member := range members {
		if err := l.svcCtx.AlertGroupMembersModel.Delete(l.ctx, member.Id); err != nil {
			l.Errorf("删除告警组成员失败 ID=%d: %v", member.Id, err)
			// 继续删除其他记录
		}
	}

	// 3. 删除所有级别渠道
	for _, channel := range channels {
		if err := l.svcCtx.AlertGroupLevelChannelsModel.Delete(l.ctx, channel.Id); err != nil {
			l.Errorf("删除告警组级别渠道失败 ID=%d: %v", channel.Id, err)
		}
	}

	// 4. 删除所有应用关联
	for _, app := range apps {
		if err := l.svcCtx.AlertGroupAppsModel.Delete(l.ctx, app.Id); err != nil {
			l.Errorf("删除告警组应用关联失败 ID=%d: %v", app.Id, err)
		}
	}

	// 5. 最后删除告警组本身
	if err := l.svcCtx.AlertGroupsModel.Delete(l.ctx, in.Id); err != nil {
		l.Errorf("删除告警组失败 ID=%d: %v", in.Id, err)
		return nil, errorx.Msg("删除告警组失败")
	}

	l.Infof("成功删除告警组 ID=%d 及其所有关联数据", in.Id)

	return &pb.DelAlertGroupsResp{}, nil
}
