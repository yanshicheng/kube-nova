package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type SetUserAdminLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetUserAdminLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetUserAdminLogic {
	return &SetUserAdminLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SetUserAdminLogic) SetUserAdmin(in *pb.SetUserAdminReq) (*pb.SetUserAdminResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	err = client.User().SetAdmin(in.UserId, in.SysadminFlag)
	if err != nil {
		return nil, errorx.Msg("设置管理员权限失败: " + err.Error())
	}

	msg := "取消管理员权限成功"
	if in.SysadminFlag {
		msg = "设置为管理员成功"
	}

	return &pb.SetUserAdminResp{Message: msg}, nil
}
