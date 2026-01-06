package billing

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingPriceConfigAddLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOnecBillingPriceConfigAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingPriceConfigAddLogic {
	return &OnecBillingPriceConfigAddLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OnecBillingPriceConfigAddLogic) OnecBillingPriceConfigAdd(req *types.OnecBillingPriceConfigAddRequest) (resp *types.OnecBillingPriceConfigAddResponse, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用RPC服务新增收费配置
	_, err = l.svcCtx.ManagerRpc.OnecBillingPriceConfigAdd(l.ctx, &pb.OnecBillingPriceConfigAddReq{
		ConfigName:    req.ConfigName,
		Description:   req.Description,
		CpuPrice:      req.CpuPrice,
		MemoryPrice:   req.MemoryPrice,
		StoragePrice:  req.StoragePrice,
		GpuPrice:      req.GpuPrice,
		PodPrice:      req.PodPrice,
		ManagementFee: req.ManagementFee,
		IsSystem:      req.IsSystem,
		CreatedBy:     username,
	})
	if err != nil {
		l.Errorf("新增收费配置失败: %v", err)
		return nil, fmt.Errorf("新增收费配置失败: %v", err)
	}

	return &types.OnecBillingPriceConfigAddResponse{}, nil
}
