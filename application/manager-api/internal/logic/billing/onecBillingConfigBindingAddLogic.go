package billing

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingConfigBindingAddLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOnecBillingConfigBindingAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingConfigBindingAddLogic {
	return &OnecBillingConfigBindingAddLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OnecBillingConfigBindingAddLogic) OnecBillingConfigBindingAdd(req *types.OnecBillingConfigBindingAddRequest) (resp *types.OnecBillingConfigBindingAddResponse, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用RPC服务新增计费配置绑定
	_, err = l.svcCtx.ManagerRpc.OnecBillingConfigBindingAdd(l.ctx, &pb.OnecBillingConfigBindingAddReq{
		BindingType:        req.BindingType,
		BindingClusterUuid: req.BindingClusterUuid,
		BindingProjectId:   req.BindingProjectId,
		PriceConfigId:      req.PriceConfigId,
		BillingStartTime:   req.BillingStartTime,
		CreatedBy:          username,
		IsGenerateBilling:  req.IsGenerateBilling,
	})
	if err != nil {
		l.Errorf("新增计费配置绑定失败: %v", err)
		return nil, fmt.Errorf("新增计费配置绑定失败: %v", err)
	}

	return &types.OnecBillingConfigBindingAddResponse{}, nil
}
