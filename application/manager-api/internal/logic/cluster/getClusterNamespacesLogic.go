package cluster

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterNamespacesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetClusterNamespacesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterNamespacesLogic {
	return &GetClusterNamespacesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterNamespacesLogic) GetClusterNamespaces(req *types.GetClusterNamespaceListRequest) (resp []string, err error) {
	ns, err := l.svcCtx.ManagerRpc.GetClusterNs(l.ctx, &managerservice.GetClusterNsReq{ClusterUuid: req.ClusterUuid})
	if err != nil {
		l.Errorf("GetClusterNs error: %v", err)
		return nil, err
	}
	return ns.Namespaces, nil
}
