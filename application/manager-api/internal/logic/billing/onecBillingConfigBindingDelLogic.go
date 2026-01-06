package billing

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingConfigBindingDelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOnecBillingConfigBindingDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingConfigBindingDelLogic {
	return &OnecBillingConfigBindingDelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OnecBillingConfigBindingDelLogic) OnecBillingConfigBindingDel(req *types.OnecBillingConfigBindingDelRequest) (resp *types.OnecBillingConfigBindingDelResponse, err error) {
	// 调用RPC服务删除计费配置绑定
	_, err = l.svcCtx.ManagerRpc.OnecBillingConfigBindingDel(l.ctx, &pb.OnecBillingConfigBindingDelReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("删除计费配置绑定失败: %v", err)
		return nil, fmt.Errorf("删除计费配置绑定失败: %v", err)
	}

	return &types.OnecBillingConfigBindingDelResponse{}, nil
}
