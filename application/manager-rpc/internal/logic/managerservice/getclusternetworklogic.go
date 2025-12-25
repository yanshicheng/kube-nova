package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterNetworkLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetClusterNetworkLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterNetworkLogic {
	return &GetClusterNetworkLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetClusterNetworkLogic) GetClusterNetwork(in *pb.GetClusterNetworkReq) (*pb.GetClusterNetworkResp, error) {
	network, err := l.svcCtx.OnecClusterNetworkModel.FindOneByClusterUuid(l.ctx, in.Uuid)
	if err != nil {
		l.Errorf("获取集群网络详情失败: %s", err.Error())
		return nil, err
	}

	return &pb.GetClusterNetworkResp{
		Id:                network.Id,
		ClusterUuid:       network.ClusterUuid,
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
		CreatedAt:         network.CreatedAt.Unix(),
		UpdatedAt:         network.UpdatedAt.Unix(),
	}, nil
}
