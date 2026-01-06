package billing

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingConfigBindingUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOnecBillingConfigBindingUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingConfigBindingUpdateLogic {
	return &OnecBillingConfigBindingUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OnecBillingConfigBindingUpdateLogic) OnecBillingConfigBindingUpdate(req *types.OnecBillingConfigBindingUpdateRequest) (resp *types.OnecBillingConfigBindingUpdateResponse, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用RPC服务更新计费配置绑定
	_, err = l.svcCtx.ManagerRpc.OnecBillingConfigBindingUpdate(l.ctx, &pb.OnecBillingConfigBindingUpdateReq{
		Id:                 req.Id,
		BindingClusterUuid: req.BindingClusterUuid,
		BindingProjectId:   req.BindingProjectId,
		PriceConfigId:      req.PriceConfigId,
		UpdatedBy:          username,
		IsGenerateBilling:  req.IsGenerateBilling,
	})
	if err != nil {
		l.Errorf("更新计费配置绑定失败: %v", err)
		return nil, fmt.Errorf("更新计费配置绑定失败: %v", err)
	}

	return &types.OnecBillingConfigBindingUpdateResponse{}, nil
}
