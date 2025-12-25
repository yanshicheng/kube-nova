package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChangeUserPasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChangeUserPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangeUserPasswordLogic {
	return &ChangeUserPasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChangeUserPasswordLogic) ChangeUserPassword(in *pb.ChangeUserPasswordReq) (*pb.ChangeUserPasswordResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	req := &types.PasswordReq{
		OldPassword: in.OldPassword,
		NewPassword: in.NewPassword,
	}

	err = client.User().ChangePassword(in.UserId, req)
	if err != nil {
		return nil, errorx.Msg("修改密码失败: " + err.Error())
	}

	return &pb.ChangeUserPasswordResp{Message: "修改密码成功"}, nil
}
