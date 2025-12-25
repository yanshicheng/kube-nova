package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserLogic {
	return &GetUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetUserLogic) GetUser(in *pb.GetUserReq) (*pb.GetUserResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	user, err := client.User().Get(in.UserId)
	if err != nil {
		return nil, errorx.Msg("查询用户失败")
	}

	return &pb.GetUserResp{
		Data: &pb.HarborUser{
			UserId:          user.UserID,
			Username:        user.Username,
			Email:           user.Email,
			Realname:        user.Realname,
			Comment:         user.Comment,
			CreationTime:    user.CreationTime.Unix(),
			UpdateTime:      user.UpdateTime.Unix(),
			SysadminFlag:    user.SysAdminFlag,
			AdminRoleInAuth: user.AdminRoleInAuth,
		},
	}, nil
}
