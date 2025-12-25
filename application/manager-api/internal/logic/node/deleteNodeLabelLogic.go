package node

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteNodeLabelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteNodeLabelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteNodeLabelLogic {
	return &DeleteNodeLabelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteNodeLabelLogic) DeleteNodeLabel(req *types.NodeLabelDeleteRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}
	_, err = l.svcCtx.ManagerRpc.NodeDeleteLabels(l.ctx, &pb.ClusterNodeDeleteLabelsReq{
		Id:        req.Id,
		Key:       req.Key,
		UpdatedBy: username,
	})
	if err != nil {
		l.Errorf("删除节点标签失败: %v", err)
		return "", errorx.Msg("删除节点标签失败")
	}

	return "操作成功", nil
}
