package cluster

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterNetworkLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetClusterNetworkLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterNetworkLogic {
	return &GetClusterNetworkLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterNetworkLogic) GetClusterNetwork(req *types.GetClusterNetworkRequest) (resp *types.ClusterNetwork, err error) {
	network, err := l.svcCtx.ManagerRpc.GetClusterNetwork(l.ctx, &managerservice.GetClusterNetworkReq{
		Uuid: req.ClusterUuid,
	})
	if err != nil {
		l.Errorf("RPC调用获取集群网络失败: %v", err)
		return nil, err
	}
	resp = l.convertClusterNetwork(network)
	return
}
func (l *GetClusterNetworkLogic) convertClusterNetwork(network *managerservice.GetClusterNetworkResp) *types.ClusterNetwork {
	return &types.ClusterNetwork{
		ClusterUuid:       network.ClusterUuid,
		Id:                network.Id,
		ClusterCidr:       network.ClusterCidr,
		ServiceCidr:       network.ServiceCidr,
		NodeCidrMaskSize:  network.NodeCidrMaskSize,
		DnsDomain:         network.DnsDomain,
		DnsServiceIp:      network.DnsServiceIp,
		DnsProvider:       network.DnsProvider,
		CniPlugin:         network.CniPlugin,
		CniVersion:        network.CniVersion,
		ProxyMode:         network.ProxyMode,
		IngressController: network.IngressController,
		IngressClass:      network.IngressClass,
		Ipv6Enabled:       network.Ipv6Enabled,
		DualStackEnabled:  network.DualStackEnabled,
		MtuSize:           network.MtuSize,
		NodePortRange:     network.NodePortRange,
		CreatedBy:         network.CreatedBy,
		UpdatedBy:         network.UpdatedBy,
		UpdatedAt:         network.UpdatedAt,
		CreatedAt:         network.CreatedAt,
	}
}
