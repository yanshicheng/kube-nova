// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package node

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNodeYamlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetNodeYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNodeYamlLogic {
	return &GetNodeYamlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNodeYamlLogic) GetNodeYaml(req *types.NodeIdRequest) (resp string, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.NodeYamlDescribe(l.ctx, &pb.ClusterNodeYamlDescribeReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取节点详情失败: %v", err)
		return
	}
	return rpcResp.Data, nil
}
