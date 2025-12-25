package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterDefaultDomainSuffixLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetClusterDefaultDomainSuffixLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterDefaultDomainSuffixLogic {
	return &GetClusterDefaultDomainSuffixLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取集群默认域名后缀
func (l *GetClusterDefaultDomainSuffixLogic) GetClusterDefaultDomainSuffix(in *pb.GetClusterDefaultDomainSuffixReq) (*pb.GetClusterDefaultDomainSuffixResp, error) {
	network, err := l.svcCtx.OnecClusterNetworkModel.FindOneByClusterUuid(l.ctx, in.Uuid)
	if err != nil {
		l.Errorf("获取集群网络详情失败: %s", err.Error())
		return nil, err
	}
	domainSuffix := "cluster.local"
	if network != nil && network.DnsDomain != "" {
		domainSuffix = network.DnsDomain
	}
	if domainSuffix[0] == '.' {
		domainSuffix = domainSuffix[1:]
	}
	return &pb.GetClusterDefaultDomainSuffixResp{
		Data: domainSuffix,
	}, nil
}
