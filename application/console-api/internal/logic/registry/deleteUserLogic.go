package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除Harbor用户
func NewDeleteUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteUserLogic {
	return &DeleteUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteUserLogic) DeleteUser(req *types.DeleteUserRequest) (resp string, err error) {
	_, err = l.svcCtx.RepositoryRpc.DeleteUser(l.ctx, &pb.DeleteUserReq{
		RegistryUuid: req.RegistryUuid,
		UserId:       req.UserId,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("删除Harbor用户成功: UserId=%d", req.UserId)
	return "删除Harbor用户成功", nil
}
