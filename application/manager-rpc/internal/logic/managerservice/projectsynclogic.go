package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectSyncLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectSyncLogic {
	return &ProjectSyncLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectSync 同步项目数据（计算资源使用情况）
func (l *ProjectSyncLogic) ProjectSync(in *pb.ProjectSyncReq) (*pb.ProjectSyncResp, error) {

	// 查询项目是否存在
	project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, in.ProjectId)
	if err != nil {
		l.Logger.Errorf("查询项目失败，ID: %d, 错误: %v", in.ProjectId, err)
		return nil, errorx.Msg("项目不存在")
	}

	// 查询项目下的所有集群配额
	clusters, err := l.svcCtx.OnecProjectClusterModel.SearchNoPage(l.ctx, "", true, "project_id = ?", in.ProjectId)
	if err != nil {
		l.Logger.Errorf("查询项目集群配额失败，项目ID: %d, 错误: %v", in.ProjectId, err)
		return nil, errorx.Msg("查询项目集群配额失败")
	}

	for _, cluster := range clusters {
		// 同步集群资源使用情况
		if err := l.svcCtx.OnecProjectModel.SyncProjectClusterResourceAllocation(l.ctx, cluster.Id); err != nil {
			l.Logger.Errorf("同步项目集群数据失败，项目集群ID: %d, 错误: %v", cluster.Id, err)
			return nil, errorx.Msg("同步项目集群数据失败")
		}
	}

	l.Logger.Infof("同步项目数据完成，项目: %s", project.Name)
	return &pb.ProjectSyncResp{}, nil
}
