package sync

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectOneClusterResourceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 同步计算项目集群的资源使用量，更新到数据库
func NewProjectOneClusterResourceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectOneClusterResourceLogic {
	return &ProjectOneClusterResourceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProjectOneClusterResourceLogic) ProjectOneClusterResource(req *types.SyncRequest) (resp string, err error) {
	// 调用RPC服务同步项目集群配额
	//通过 resourceId 查询 project id
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}
	// 启动 go 协程
	go func() {
		ctx := context.Background()
		_, err = l.svcCtx.ManagerRpc.ProjectClusterSync(ctx, &pb.ProjectClusterSyncReq{
			Id:       req.Id,
			Operator: username,
		})
		if err != nil {
			l.Errorf("同步项目集群资源使用量失败: %v", err)
			return
		}
	}()

	return "正在异步同步", nil
}
