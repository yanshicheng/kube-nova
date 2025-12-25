package sync

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectWorkspaceSyncLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 同步工作空间的实际运行状态和资源使用情况
func NewProjectWorkspaceSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectWorkspaceSyncLogic {
	return &ProjectWorkspaceSyncLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProjectWorkspaceSyncLogic) ProjectWorkspaceSync(req *types.SyncRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	go func() {
		ctx := context.Background()
		_, err = l.svcCtx.ManagerRpc.ProjectWorkspaceSync(ctx, &pb.ProjectWorkspaceSyncReq{
			Id:       req.Id,
			Operator: username,
		})

		if err != nil {
			l.Errorf("同步项目工作空间状态失败: %v", err)
			return
		}

		l.Infof("项目工作空间状态同步成功, ID: %d", req.Id)
	}()
	return "项目工作空间状态同步成功", nil
}
