package sync

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterResourceSyncLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewClusterResourceSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterResourceSyncLogic {
	return &ClusterResourceSyncLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterResourceSyncLogic) ClusterResourceSync(req *types.SyncClusterResourceRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}
	// 启动 go 协程
	go func() {
		ctx := context.Background()
		_, err = l.svcCtx.ManagerRpc.ClusterResourceSync(ctx, &managerservice.ClusterResourceSyncReq{
			Id:       req.Id,
			Operator: username,
		})
		if err != nil {
			l.Errorf("同步集群资源失败: %v", err)
			return
		}
		l.Infof("集群资源同步成功, ID: %d", req.Id)
	}()
	return "正在异步同步", nil
}
