package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建Harbor用户
func NewCreateUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateUserLogic {
	return &CreateUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateUserLogic) CreateUser(req *types.CreateUserRequest) (resp string, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.CreateUser(l.ctx, &pb.CreateUserReq{
		RegistryUuid: req.RegistryUuid,
		Username:     req.Username,
		Email:        req.Email,
		Realname:     req.Realname,
		Password:     req.Password,
		Comment:      req.Comment,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("创建Harbor用户成功: UserId=%d, Username=%s", rpcResp.UserId, req.Username)
	return "创建Harbor用户成功", nil
}
