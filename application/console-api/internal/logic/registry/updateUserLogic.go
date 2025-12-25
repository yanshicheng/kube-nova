package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新Harbor用户
func NewUpdateUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUserLogic {
	return &UpdateUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateUserLogic) UpdateUser(req *types.UpdateUserRequest) (resp string, err error) {
	_, err = l.svcCtx.RepositoryRpc.UpdateUser(l.ctx, &pb.UpdateUserReq{
		RegistryUuid: req.RegistryUuid,
		UserId:       req.UserId,
		Email:        req.Email,
		Realname:     req.Realname,
		Comment:      req.Comment,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("更新Harbor用户成功: UserId=%d", req.UserId)
	return "更新Harbor用户成功", nil
}
