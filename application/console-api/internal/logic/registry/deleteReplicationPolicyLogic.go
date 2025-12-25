package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteReplicationPolicyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除复制策略
func NewDeleteReplicationPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteReplicationPolicyLogic {
	return &DeleteReplicationPolicyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteReplicationPolicyLogic) DeleteReplicationPolicy(req *types.DeleteReplicationPolicyRequest) (resp string, err error) {
	_, err = l.svcCtx.RepositoryRpc.DeleteReplicationPolicy(l.ctx, &pb.DeleteReplicationPolicyReq{
		RegistryUuid: req.RegistryUuid,
		PolicyId:     req.PolicyId,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("复制策略删除成功: PolicyId=%d", req.PolicyId)
	return "复制策略删除成功", nil
}
