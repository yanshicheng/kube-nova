package sync

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterAllSyncLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewClusterAllSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterAllSyncLogic {
	return &ClusterAllSyncLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterAllSyncLogic) ClusterAllSync() (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}
	// 启动 go 协程
	go func() {
		ctx := context.Background()
		_, err = l.svcCtx.ManagerRpc.ClusterSyncAll(ctx, &managerservice.ClusterSyncAllReq{
			Operator: username,
		})
		if err != nil {
			l.Errorf("同步所有集群失败: %v", err)
		}
	}()
	return "同步所有集群成功", nil
}
