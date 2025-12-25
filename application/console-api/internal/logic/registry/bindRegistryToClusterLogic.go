package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type BindRegistryToClusterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 绑定仓库到集群
func NewBindRegistryToClusterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindRegistryToClusterLogic {
	return &BindRegistryToClusterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BindRegistryToClusterLogic) BindRegistryToCluster(req *types.BindRegistryToClusterRequest) (resp string, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.BindRegistryToCluster(l.ctx, &pb.BindRegistryToClusterReq{
		RegistryId:  req.RegistryId,
		ClusterUuid: req.ClusterUuid,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("绑定仓库到集群成功: BindingId=%d", rpcResp.Id)
	return "绑定仓库到集群成功", nil

}
