package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取Harbor用户详情
func NewGetUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserLogic {
	return &GetUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUserLogic) GetUser(req *types.GetUserRequest) (resp *types.GetUserResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.GetUser(l.ctx, &pb.GetUserReq{
		RegistryUuid: req.RegistryUuid,
		UserId:       req.UserId,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	data := rpcResp.Data
	l.Infof("获取Harbor用户详情成功: UserId=%d, Username=%s", data.UserId, data.Username)

	return &types.GetUserResponse{
		Data: types.HarborUser{
			UserId:          data.UserId,
			Username:        data.Username,
			Email:           data.Email,
			Realname:        data.Realname,
			Comment:         data.Comment,
			CreationTime:    data.CreationTime,
			UpdateTime:      data.UpdateTime,
			SysadminFlag:    data.SysadminFlag,
			AdminRoleInAuth: data.AdminRoleInAuth,
		},
	}, nil
}
