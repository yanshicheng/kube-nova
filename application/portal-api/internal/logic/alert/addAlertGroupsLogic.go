package alert

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddAlertGroupsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddAlertGroupsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddAlertGroupsLogic {
	return &AddAlertGroupsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddAlertGroupsLogic) AddAlertGroups(req *types.AddAlertGroupsRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	l.Infof("添加告警组请求: operator=%s, groupName=%s", username, req.GroupName)

	// 转换用户列表
	users := make([]*pb.AddRAlertGroupUserReq, 0, len(req.Users))
	for _, user := range req.Users {
		users = append(users, &pb.AddRAlertGroupUserReq{
			UserId: user.UserId,
			Role:   user.Role,
		})
	}

	// 转换渠道列表
	channels := make([]*pb.AddGroupLevelChannelsReq, 0, len(req.Channels))
	for _, channel := range req.Channels {
		channels = append(channels, &pb.AddGroupLevelChannelsReq{
			Severity:  channel.Severity,
			ChannelId: channel.ChannelId,
		})
	}

	// 调用 RPC 服务添加告警组
	_, err = l.svcCtx.AlertPortalRpc.AlertGroupsAdd(l.ctx, &pb.AddAlertGroupsReq{
		GroupName:    req.GroupName,
		GroupType:    req.GroupType,
		Description:  req.Description,
		FilterRules:  req.FilterRules,
		DutySchedule: req.DutySchedule,
		SortOrder:    req.SortOrder,
		Users:        users,
		Channels:     channels,
		CreatedBy:    username,
		UpdatedBy:    username,
	})
	if err != nil {
		l.Errorf("添加告警组失败: operator=%s, groupName=%s, error=%v", username, req.GroupName, err)
		return "", err
	}

	l.Infof("添加告警组成功: operator=%s, groupName=%s", username, req.GroupName)
	return "添加告警组成功", nil
}
