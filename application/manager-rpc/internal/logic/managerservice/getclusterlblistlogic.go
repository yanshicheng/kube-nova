package managerservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterLbListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetClusterLbListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterLbListLogic {
	return &GetClusterLbListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取集群 node or master lb 列表
func (l *GetClusterLbListLogic) GetClusterLbList(in *pb.GetClusterLbListReq) (*pb.GetClusterLbListResp, error) {
	cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, in.Uuid)
	if err != nil {
		logx.Error("查询集群数据失败", err)
		return nil, errorx.Msg("查询集群数据失败")
	}
	var lbList []string
	switch strings.ToLower(in.LbName) {
	case "node":
		if cluster.NodeLb != "" {
			lbList = strings.Split(cluster.NodeLb, ",")
		}
	case "master":
		if cluster.MasterLb != "" {
			lbList = strings.Split(cluster.MasterLb, ",")
		}

	}
	return &pb.GetClusterLbListResp{
		Data: lbList,
	}, nil
}
