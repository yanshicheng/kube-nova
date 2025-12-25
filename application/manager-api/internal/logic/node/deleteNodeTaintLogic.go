package node

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteNodeTaintLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteNodeTaintLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteNodeTaintLogic {
	return &DeleteNodeTaintLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteNodeTaintLogic) DeleteNodeTaint(req *types.NodeTaintDeleteRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}
	_, err = l.svcCtx.ManagerRpc.NodeDeleteTaint(l.ctx, &pb.ClusterNodeDeleteTaintReq{
		Id:        req.Id,
		Key:       req.Key,
		Effect:    req.Effect,
		UpdatedBy: username,
	})
	if err != nil {
		l.Errorf("删除节点污点失败: %v", err)
		return "", errorx.Msg("删除节点污点失败")
	}

	return "操作成功", nil
}
