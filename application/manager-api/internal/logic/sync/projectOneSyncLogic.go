package sync

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectOneSyncLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 同步项目相关数据
func NewProjectOneSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectOneSyncLogic {
	return &ProjectOneSyncLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProjectOneSyncLogic) ProjectOneSync(req *types.SyncRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}
	// go 协程
	go func() {
		// 调用RPC服务同步项目
		ctx := context.Background()
		_, err = l.svcCtx.ManagerRpc.ProjectSync(ctx, &pb.ProjectSyncReq{
			ProjectId: req.Id,
			Operator:  username,
		})

		if err != nil {
			l.Errorf("同步项目失败: %v", err)
			return
		}

	}()

	return "项目同步成功", nil
}
