package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type UnbindRegistryFromClusterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 解绑仓库与集群
func NewUnbindRegistryFromClusterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnbindRegistryFromClusterLogic {
	return &UnbindRegistryFromClusterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UnbindRegistryFromClusterLogic) UnbindRegistryFromCluster(req *types.UnbindRegistryFromClusterRequest) (resp string, err error) {
	_, err = l.svcCtx.RepositoryRpc.UnbindRegistryFromCluster(l.ctx, &pb.UnbindRegistryFromClusterReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("解绑仓库与集群成功: ID=%d", req.Id)
	return "解绑仓库与集群成功", nil
}
