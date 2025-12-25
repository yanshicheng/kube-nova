package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteReplicationPolicyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteReplicationPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteReplicationPolicyLogic {
	return &DeleteReplicationPolicyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteReplicationPolicyLogic) DeleteReplicationPolicy(in *pb.DeleteReplicationPolicyReq) (*pb.DeleteReplicationPolicyResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	err = client.Replication().Delete(in.PolicyId)
	if err != nil {
		return nil, errorx.Msg("删除复制策略失败")
	}

	return &pb.DeleteReplicationPolicyResp{
		Message: "复制策略删除成功",
	}, nil
}
