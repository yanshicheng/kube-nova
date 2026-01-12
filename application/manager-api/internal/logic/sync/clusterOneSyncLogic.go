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
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}
	l.Infof("用户 [%s] 请求同步集群 [ID=%d]", username, req.Id)

	_, err = l.svcCtx.ManagerRpc.ClusterSync(l.ctx, &pb.SyncClusterReq{
		Id:        req.Id,
		UpdatedBy: username,
	})
	if err != nil {
		l.Errorf("提交同步请求失败: %v", err)
		return "", err
	}

	return "集群同步任务已提交", nil
}
