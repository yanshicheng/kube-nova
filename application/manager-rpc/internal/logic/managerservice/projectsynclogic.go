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

func (l *ProjectSyncLogic) ProjectSync(in *pb.ProjectSyncReq) (*pb.ProjectSyncResp, error) {
	project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, in.ProjectId)
	if err != nil {
		l.Logger.Errorf("查询项目失败，ID: %d, 错误: %v", in.ProjectId, err)
		return nil, errorx.Msg("项目不存在")
	}

	projectId := in.ProjectId
	projectName := project.Name
	svcCtx := l.svcCtx

	go func() {
		ctx := context.Background()
		logger := logx.WithContext(ctx)

		logger.Infof("开始异步同步项目 [%s] 数据", projectName)

		// 查询项目下的所有集群配额
		clusters, err := svcCtx.OnecProjectClusterModel.SearchNoPage(ctx, "", true, "project_id = ?", projectId)
		if err != nil {
			logger.Errorf("查询项目集群配额失败，项目ID: %d, 错误: %v", projectId, err)
			return
		}

		for _, cluster := range clusters {
			if err := svcCtx.OnecProjectModel.SyncProjectClusterResourceAllocation(ctx, cluster.Id); err != nil {
				logger.Errorf("同步项目集群数据失败，项目集群ID: %d, 错误: %v", cluster.Id, err)
				// 继续处理其他集群
				continue
			}
		}

		logger.Infof("项目 [%s] 数据同步完成", projectName)
	}()

	return &pb.ProjectSyncResp{}, nil
}
