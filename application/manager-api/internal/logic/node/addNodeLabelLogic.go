package node

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddNodeLabelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddNodeLabelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddNodeLabelLogic {
	return &AddNodeLabelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddNodeLabelLogic) AddNodeLabel(req *types.NodeLabelRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}
	_, err = l.svcCtx.ManagerRpc.NodeAddLabels(l.ctx, &pb.ClusterNodeAddLabelsReq{
		Id:        req.Id,
		Key:       req.Key,
		Value:     req.Value,
		UpdatedBy: username,
	})
	if err != nil {
		l.Errorf("添加节点标签失败: %v", err)
		return "", errorx.Msg("添加节点标签失败")
	}

	return "操作成功", nil
}
