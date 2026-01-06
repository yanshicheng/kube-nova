package billing

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingPriceConfigUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOnecBillingPriceConfigUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingPriceConfigUpdateLogic {
	return &OnecBillingPriceConfigUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OnecBillingPriceConfigUpdateLogic) OnecBillingPriceConfigUpdate(req *types.OnecBillingPriceConfigUpdateRequest) (resp *types.OnecBillingPriceConfigUpdateResponse, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用RPC服务更新收费配置
	_, err = l.svcCtx.ManagerRpc.OnecBillingPriceConfigUpdate(l.ctx, &pb.OnecBillingPriceConfigUpdateReq{
		Id:                req.Id,
		ConfigName:        req.ConfigName,
		Description:       req.Description,
		CpuPrice:          req.CpuPrice,
		MemoryPrice:       req.MemoryPrice,
		StoragePrice:      req.StoragePrice,
		GpuPrice:          req.GpuPrice,
		PodPrice:          req.PodPrice,
		ManagementFee:     req.ManagementFee,
		UpdatedBy:         username,
		IsGenerateBilling: req.IsGenerateBilling,
	})
	if err != nil {
		l.Errorf("更新收费配置失败: %v", err)
		return nil, fmt.Errorf("更新收费配置失败: %v", err)
	}

	return &types.OnecBillingPriceConfigUpdateResponse{}, nil
}
