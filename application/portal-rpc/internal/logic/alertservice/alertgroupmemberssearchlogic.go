package alertservicelogic

import (
	"context"
	"errors"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertGroupMembersSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertGroupMembersSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertGroupMembersSearchLogic {
	return &AlertGroupMembersSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertGroupMembersSearch 搜索告警组成员（关联用户表）
func (l *AlertGroupMembersSearchLogic) AlertGroupMembersSearch(in *pb.SearchAlertGroupMembersReq) (*pb.SearchAlertGroupMembersResp, error) {
	var queryStr string
	var args []interface{}
	conditions := []string{}

	// 构建查询条件
	if in.GroupId != 0 {
		conditions = append(conditions, "`group_id` = ?")
		args = append(args, in.GroupId)
	}

	if in.UserId != 0 {
		conditions = append(conditions, "`user_id` = ?")
		args = append(args, in.UserId)
	}

	if in.Role != "" {
		conditions = append(conditions, "`role` = ?")
		args = append(args, in.Role)
	}

	if len(conditions) > 0 {
		queryStr = fmt.Sprintf("%s", joinConditions(conditions, " AND "))
	}

	// 查询告警组成员
	dataList, total, err := l.svcCtx.AlertGroupMembersModel.Search(
		l.ctx,
		in.OrderStr,
		in.IsAsc,
		in.Page,
		in.PageSize,
		queryStr,
		args...,
	)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Infof("未找到匹配的告警组成员")
			return &pb.SearchAlertGroupMembersResp{
				Data:  make([]*pb.AlertGroupMembers, 0),
				Total: 0,
			}, nil
		}
		l.Errorf("搜索告警组成员失败: %v", err)
		return nil, errorx.Msg("搜索告警组成员失败")
	}

	// 转换数据并关联用户信息
	list := make([]*pb.AlertGroupMembers, 0, len(dataList))
	for _, data := range dataList {
		member := &pb.AlertGroupMembers{
			Id:        data.Id,
			GroupId:   data.GroupId,
			UserId:    data.UserId,
			Role:      data.Role,
			CreatedBy: data.CreatedBy,
			UpdatedBy: data.UpdatedBy,
			CreatedAt: data.CreatedAt.Unix(),
			UpdatedAt: data.UpdatedAt.Unix(),
		}

		// 关联查询用户信息
		user, err := l.svcCtx.SysUser.FindOne(l.ctx, data.UserId)
		if err != nil {
			l.Errorf("查询用户信息失败: userId=%d, err=%v", data.UserId, err)
			member.UserName = ""
			member.UserAccount = ""
		} else {
			member.UserName = user.Nickname
			member.UserAccount = user.Username
			l.Infof("关联用户信息: userId=%d, nickname=%s, username=%s",
				data.UserId, user.Nickname, user.Username)
		}
		list = append(list, member)
	}

	l.Infof("查询到 %d 条告警组成员记录，总数: %d", len(list), total)
	return &pb.SearchAlertGroupMembersResp{
		Data:  list,
		Total: total,
	}, nil
}
