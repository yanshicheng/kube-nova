package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type SetUserAdminLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 设置用户管理员权限
func NewSetUserAdminLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetUserAdminLogic {
	return &SetUserAdminLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SetUserAdminLogic) SetUserAdmin(req *types.SetUserAdminRequest) (resp string, err error) {
	_, err = l.svcCtx.RepositoryRpc.SetUserAdmin(l.ctx, &pb.SetUserAdminReq{
		RegistryUuid: req.RegistryUuid,
		UserId:       req.UserId,
		SysadminFlag: req.SysadminFlag,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	action := "取消管理员权限"
	if req.SysadminFlag {
		action = "设置为管理员"
	}
	l.Infof("%s成功: UserId=%d", action, req.UserId)
	return action + "成功", nil
}
