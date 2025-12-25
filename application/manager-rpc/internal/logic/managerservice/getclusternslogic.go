package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterNsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetClusterNsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterNsLogic {
	return &GetClusterNsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取某一个集群的所有ns
func (l *GetClusterNsLogic) GetClusterNs(in *pb.GetClusterNsReq) (*pb.GetClusterNsResp, error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, in.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, errorx.Msg("获取集群客户端失败")
	}
	listNs, err := client.Namespaces().ListAll()
	if err != nil {
		l.Errorf("获取命名空间列表失败: %v", err)
		return nil, errorx.Msg("获取命名空间列表失败")
	}
	var namespaces []string
	for _, ns := range listNs {

		namespaces = append(namespaces, ns.Name)
	}
	return &pb.GetClusterNsResp{
		Namespaces: namespaces,
	}, nil
}
