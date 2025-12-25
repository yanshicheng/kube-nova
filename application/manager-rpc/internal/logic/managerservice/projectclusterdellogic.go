package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectClusterDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectClusterDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectClusterDelLogic {
	return &ProjectClusterDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectClusterDel 删除项目集群配额

// ProjectClusterDel 删除项目集群配额
func (l *ProjectClusterDelLogic) ProjectClusterDel(in *pb.DelOnecProjectClusterReq) (*pb.DelOnecProjectClusterResp, error) {
	l.Logger.Infof("开始删除项目集群配额，ID: %d", in.Id)

	// 查询项目集群配额是否存在
	_, err := l.svcCtx.OnecProjectClusterModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("查询项目集群配额失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("项目集群配额不存在")
	}

	// 检查是否有工作空间关联
	workspaces, err := l.svcCtx.OnecProjectWorkspaceModel.SearchNoPage(l.ctx, "", true, "project_cluster_id = ?", in.Id)
	if err == nil && len(workspaces) > 0 {
		l.Logger.Error("项目集群配额下存在工作空间，无法删除，ID: %d", in.Id)
		return nil, errorx.Msg("项目集群配额下存在工作空间，请先删除相关工作空间")
	}

	// 执行硬删除
	err = l.svcCtx.OnecProjectClusterModel.Delete(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("删除项目集群配额失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("删除项目集群配额失败")
	}

	l.Logger.Infof("删除项目集群配额成功，ID: %d", in.Id)
	return &pb.DelOnecProjectClusterResp{}, nil
}
