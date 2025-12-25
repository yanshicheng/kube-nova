package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateUserLogic {
	return &CreateUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateUser 创建 Harbor 用户
func (l *CreateUserLogic) CreateUser(in *pb.CreateUserReq) (*pb.CreateUserResp, error) {
	// 获取 Harbor 客户端
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	// 构建请求
	req := &types.UserReq{
		Username: in.Username,
		Email:    in.Email,
		Realname: in.Realname,
		Password: in.Password,
		Comment:  in.Comment,
	}

	// 创建用户
	userID, err := client.User().Create(req)
	if err != nil {
		return nil, errorx.Msg("创建用户失败: " + err.Error())
	}

	return &pb.CreateUserResp{
		UserId:  userID,
		Message: "创建成功",
	}, nil
}
