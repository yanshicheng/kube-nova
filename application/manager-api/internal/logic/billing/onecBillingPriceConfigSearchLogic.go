package billing

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingPriceConfigSearchLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOnecBillingPriceConfigSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingPriceConfigSearchLogic {
	return &OnecBillingPriceConfigSearchLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OnecBillingPriceConfigSearchLogic) OnecBillingPriceConfigSearch(req *types.OnecBillingPriceConfigSearchRequest) (resp *types.OnecBillingPriceConfigSearchResponse, err error) {
	// 调用RPC服务搜索收费配置
	result, err := l.svcCtx.ManagerRpc.OnecBillingPriceConfigSearch(l.ctx, &pb.OnecBillingPriceConfigSearchReq{
		ConfigName: req.ConfigName,
	})
	if err != nil {
		l.Errorf("搜索收费配置失败: %v", err)
		return nil, fmt.Errorf("搜索收费配置失败: %v", err)
	}

	// 转换响应数据
	var data []types.OnecBillingPriceConfig
	for _, item := range result.Data {
		data = append(data, types.OnecBillingPriceConfig{
			Id:            item.Id,
			ConfigName:    item.ConfigName,
			Description:   item.Description,
			CpuPrice:      item.CpuPrice,
			MemoryPrice:   item.MemoryPrice,
			StoragePrice:  item.StoragePrice,
			GpuPrice:      item.GpuPrice,
			PodPrice:      item.PodPrice,
			ManagementFee: item.ManagementFee,
			IsSystem:      item.IsSystem,
			CreatedBy:     item.CreatedBy,
			UpdatedBy:     item.UpdatedBy,
			CreatedAt:     item.CreatedAt,
			UpdatedAt:     item.UpdatedAt,
		})
	}

	return &types.OnecBillingPriceConfigSearchResponse{
		Data: data,
	}, nil
}
