package alertservicelogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertGroupsAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertGroupsAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertGroupsAddLogic {
	return &AlertGroupsAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// -----------------------告警组表-----------------------
func (l *AlertGroupsAddLogic) AlertGroupsAdd(in *pb.AddAlertGroupsReq) (*pb.AddAlertGroupsResp, error) {
	// 生成UUID
	groupUUID := uuid.New().String()

	// 处理JSON字段：空字符串转为 JSON null
	filterRules := in.FilterRules
	if filterRules == "" {
		filterRules = "{}" // JSON null值
	}

	dutySchedule := in.DutySchedule
	if dutySchedule == "" {
		dutySchedule = "{}" // JSON null值
	}

	// 使用事务处理
	err := l.svcCtx.AlertGroupsModel.TransCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		// 1. 插入告警组 - 注意第一个括号的位置
		result, err := session.ExecCtx(ctx,
			"INSERT INTO `alert_groups` (`uuid`, `group_name`, `group_type`, `description`, `filter_rules`, `duty_schedule`, `sort_order`, `created_by`, `updated_by`, `is_deleted`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			groupUUID, in.GroupName, in.GroupType, in.Description,
			filterRules, dutySchedule, in.SortOrder,
			in.CreatedBy, in.UpdatedBy, 0)
		if err != nil {
			return err
		}

		groupId, err := result.LastInsertId()
		if err != nil {
			return err
		}

		// 2. 插入告警组成员
		if in.Users != nil && len(in.Users) > 0 {
			for _, user := range in.Users {
				_, err := session.ExecCtx(ctx,
					"INSERT INTO `alert_group_members` (`group_id`, `user_id`, `role`, `created_by`, `updated_by`, `is_deleted`) VALUES (?, ?, ?, ?, ?, ?)",
					groupId, user.UserId, user.Role,
					in.CreatedBy, in.UpdatedBy, 0)
				if err != nil {
					return err
				}
			}
		}

		// 3. 插入告警组级别渠道关联
		if in.Channels != nil && len(in.Channels) > 0 {
			for _, channel := range in.Channels {
				_, err := session.ExecCtx(ctx,
					"INSERT INTO `alert_group_level_channels` (`group_id`, `severity`, `channel_id`, `created_by`, `updated_by`, `is_deleted`) VALUES (?, ?, ?, ?, ?, ?)",
					groupId, channel.Severity, channel.ChannelId,
					in.CreatedBy, in.UpdatedBy, 0)
				if err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		l.Error("事务执行失败", err)
		return nil, errorx.Msg("添加告警组失败")
	}

	return &pb.AddAlertGroupsResp{}, nil
}
