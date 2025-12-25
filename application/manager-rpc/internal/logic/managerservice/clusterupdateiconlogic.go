package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterUpdateIconLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClusterUpdateIconLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterUpdateIconLogic {
	return &ClusterUpdateIconLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 修改集群图标
func (l *ClusterUpdateIconLogic) ClusterUpdateIcon(in *pb.ClusterUpdateAvatarReq) (*pb.ClusterUpdateAvatarResp, error) {

	// 查询集群数据
	cluster, err := l.svcCtx.OnecClusterModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("查询集群数据失败: %v", err)
		return nil, err
	}
	cluster.Avatar = in.Avatar
	if err := l.svcCtx.OnecClusterModel.Update(l.ctx, cluster); err != nil {
		l.Errorf("更新集群数据失败: %v", err)
		return nil, errorx.Msg("更新集群数据失败")
	}
	return &pb.ClusterUpdateAvatarResp{}, nil
}
