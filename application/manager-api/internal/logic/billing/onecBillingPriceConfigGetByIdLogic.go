package billing

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingPriceConfigGetByIdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOnecBillingPriceConfigGetByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingPriceConfigGetByIdLogic {
	return &OnecBillingPriceConfigGetByIdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OnecBillingPriceConfigGetByIdLogic) OnecBillingPriceConfigGetById(req *types.OnecBillingPriceConfigGetByIdRequest) (resp *types.OnecBillingPriceConfigGetByIdResponse, err error) {
	// 调用RPC服务获取收费配置
	result, err := l.svcCtx.ManagerRpc.OnecBillingPriceConfigGetById(l.ctx, &pb.OnecBillingPriceConfigGetByIdReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取收费配置失败: %v", err)
		return nil, fmt.Errorf("获取收费配置失败: %v", err)
	}

	// 转换响应数据
	return &types.OnecBillingPriceConfigGetByIdResponse{
		Data: types.OnecBillingPriceConfig{
			Id:            result.Data.Id,
			ConfigName:    result.Data.ConfigName,
			Description:   result.Data.Description,
			CpuPrice:      result.Data.CpuPrice,
			MemoryPrice:   result.Data.MemoryPrice,
			StoragePrice:  result.Data.StoragePrice,
			GpuPrice:      result.Data.GpuPrice,
			PodPrice:      result.Data.PodPrice,
			ManagementFee: result.Data.ManagementFee,
			IsSystem:      result.Data.IsSystem,
			CreatedBy:     result.Data.CreatedBy,
			UpdatedBy:     result.Data.UpdatedBy,
			CreatedAt:     result.Data.CreatedAt,
			UpdatedAt:     result.Data.UpdatedAt,
		},
	}, nil
}
