package cluster

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterIngressDomainsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetClusterIngressDomainsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterIngressDomainsLogic {
	return &GetClusterIngressDomainsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterIngressDomainsLogic) GetClusterIngressDomains(req *types.GetClusterIngressDomainsRequest) (resp []string, err error) {
	res, err := l.svcCtx.ManagerRpc.GetClusterIngressDomainList(l.ctx, &managerservice.GetClusterDomainListReq{
		Uuid: req.ClusterUuid,
	})
	if err != nil {
		l.Errorf("RPC调用获取集群Ingress域名列表失败: %v", err)
		return nil, err
	}
	resp = res.Data
	return
}
