package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteRegistryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除镜像仓库
func NewDeleteRegistryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteRegistryLogic {
	return &DeleteRegistryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteRegistryLogic) DeleteRegistry(req *types.DeleteRegistryRequest) (resp string, err error) {
	_, err = l.svcCtx.RepositoryRpc.DeleteRegistry(l.ctx, &pb.DeleteRegistryReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("镜像仓库删除成功: ID=%d", req.Id)
	return "镜像仓库删除成功", nil

}
