package node

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type EvictNodeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewEvictNodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EvictNodeLogic {
	return &EvictNodeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EvictNodeLogic) EvictNode(req *types.NodeEvictRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}
	_, err = l.svcCtx.ManagerRpc.NodeEvict(l.ctx, &pb.ClusterEvictReq{
		Id:        req.Id,
		UpdatedBy: username,
	})
	if err != nil {
		l.Errorf("驱逐节点失败: %v", err)
		return "", errorx.Msg("驱逐节点失败")
	}

	return "操作成功", nil
}
