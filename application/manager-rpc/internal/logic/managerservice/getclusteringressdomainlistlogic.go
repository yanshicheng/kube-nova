package managerservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterIngressDomainListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetClusterIngressDomainListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterIngressDomainListLogic {
	return &GetClusterIngressDomainListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取 某一个集群 ingressDomain 列表
func (l *GetClusterIngressDomainListLogic) GetClusterIngressDomainList(in *pb.GetClusterDomainListReq) (*pb.GetClusterDomainListResp, error) {
	cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, in.Uuid)
	if err != nil {
		l.Errorf("查询集群失败: %v", err)
		return nil, err
	}
	// 如果为空直接返回[] 如果存在 , 号分隔返回 []strings
	domains := strings.Split(cluster.IngressDomain, ",")
	for i := range domains {
		domains[i] = strings.TrimSpace(domains[i])
	}
	return &pb.GetClusterDomainListResp{
		Data: domains,
	}, nil
}
