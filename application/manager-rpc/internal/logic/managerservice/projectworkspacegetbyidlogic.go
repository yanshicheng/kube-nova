package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectWorkspaceGetByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectWorkspaceGetByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectWorkspaceGetByIdLogic {
	return &ProjectWorkspaceGetByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectWorkspaceGetById 根据ID获取工作空间详情
func (l *ProjectWorkspaceGetByIdLogic) ProjectWorkspaceGetById(in *pb.GetOnecProjectWorkspaceByIdReq) (*pb.GetOnecProjectWorkspaceByIdResp, error) {

	workspace, err := l.svcCtx.OnecProjectWorkspaceModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("查询工作空间失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("工作空间不存在")
	}

	// 查询集群名称
	cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, workspace.ClusterUuid)
	clusterName := ""
	if err == nil {
		clusterName = cluster.Name
	}
	return &pb.GetOnecProjectWorkspaceByIdResp{
		Data: convertWorkspaceToProto(workspace, clusterName),
	}, nil
}
