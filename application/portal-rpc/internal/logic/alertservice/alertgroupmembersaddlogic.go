package alertservicelogic

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertGroupMembersAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertGroupMembersAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertGroupMembersAddLogic {
	return &AlertGroupMembersAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// -----------------------告警组成员表-----------------------
func (l *AlertGroupMembersAddLogic) AlertGroupMembersAdd(in *pb.AddAlertGroupMembersReq) (*pb.AddAlertGroupMembersResp, error) {
	data := &model.AlertGroupMembers{
		GroupId:   in.GroupId,
		UserId:    in.UserId,
		Role:      in.Role,
		CreatedBy: in.CreatedBy,
		UpdatedBy: in.UpdatedBy,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		IsDeleted: 0,
	}

	_, err := l.svcCtx.AlertGroupMembersModel.Insert(l.ctx, data)
	if err != nil {
		return nil, errorx.Msg("添加告警组成员失败")
	}

	return &pb.AddAlertGroupMembersResp{}, nil
}
