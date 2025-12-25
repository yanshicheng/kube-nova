package node

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddNodeTaintLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddNodeTaintLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddNodeTaintLogic {
	return &AddNodeTaintLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddNodeTaintLogic) AddNodeTaint(req *types.NodeTaintRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}
	_, err = l.svcCtx.ManagerRpc.NodeAddTaint(l.ctx, &pb.ClusterNodeAddTaintReq{
		Id:        req.Id,
		Key:       req.Key,
		Value:     req.Value,
		Effect:    req.Effect,
		UpdatedBy: username,
	})
	if err != nil {
		l.Errorf("添加节点污点失败: %v", err)
		return "", errorx.Msg("添加节点污点失败")
	}

	return "操作成功", nil
}
