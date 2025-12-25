package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ChangeUserPasswordLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 修改用户密码
func NewChangeUserPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangeUserPasswordLogic {
	return &ChangeUserPasswordLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ChangeUserPasswordLogic) ChangeUserPassword(req *types.ChangeUserPasswordRequest) (resp string, err error) {
	_, err = l.svcCtx.RepositoryRpc.ChangeUserPassword(l.ctx, &pb.ChangeUserPasswordReq{
		RegistryUuid: req.RegistryUuid,
		UserId:       req.UserId,
		OldPassword:  req.OldPassword,
		NewPassword:  req.NewPassword,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("修改用户密码成功: UserId=%d", req.UserId)
	return "修改用户密码成功", nil
}
