package sync

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectAllClusterResourceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 同步所有项目集群配额
func NewProjectAllClusterResourceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectAllClusterResourceLogic {
	return &ProjectAllClusterResourceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProjectAllClusterResourceLogic) ProjectAllClusterResource() (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}
	// 启动 go 协程
	go func() {
		ctx := context.Background()
		_, err = l.svcCtx.ManagerRpc.ProjectAllSync(ctx, &managerservice.ProjectQuotaSyncReq{
			Operator: username,
		})
		if err != nil {
			l.Errorf("同步所有项目集群配额失败: %v", err)
			return
		}
	}()
	return "正在异步同步", nil
}
