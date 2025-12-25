package sync

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterOneSyncLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewClusterOneSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterOneSyncLogic {
	return &ClusterOneSyncLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterOneSyncLogic) ClusterOneSync(req *types.SyncClusterRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}
	l.Infof("用户 [%s] 开始同步集群 [ID=%d]", username, req.Id)

	// 调用RPC同步集群
	// 	启动 go 协程
	go func() {
		ctx := context.Background()
		_, err = l.svcCtx.ManagerRpc.ClusterSync(ctx, &pb.SyncClusterReq{
			Id:        req.Id,
			UpdatedBy: username,
		})
		if err != nil {
			l.Errorf("RPC调用同步集群失败: %v", err)
		}
	}()
	return "集群正在同步成功", nil
}
