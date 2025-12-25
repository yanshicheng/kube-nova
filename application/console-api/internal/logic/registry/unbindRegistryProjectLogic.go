package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type UnbindRegistryProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 解绑仓库项目
func NewUnbindRegistryProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnbindRegistryProjectLogic {
	return &UnbindRegistryProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UnbindRegistryProjectLogic) UnbindRegistryProject(req *types.UnbindRegistryProjectRequest) (resp string, err error) {
	_, err = l.svcCtx.RepositoryRpc.UnbindRegistryProject(l.ctx, &pb.UnbindRegistryProjectReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("解绑仓库项目成功: ID=%d", req.Id)
	return "", nil
}
