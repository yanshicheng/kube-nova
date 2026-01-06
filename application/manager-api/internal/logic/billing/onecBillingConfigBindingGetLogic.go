package billing

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingConfigBindingGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOnecBillingConfigBindingGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingConfigBindingGetLogic {
	return &OnecBillingConfigBindingGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OnecBillingConfigBindingGetLogic) OnecBillingConfigBindingGet(req *types.OnecBillingConfigBindingGetRequest) (resp *types.OnecBillingConfigBindingGetResponse, err error) {
	// 调用RPC服务获取计费配置绑定
	result, err := l.svcCtx.ManagerRpc.OnecBillingConfigBindingGet(l.ctx, &pb.OnecBillingConfigBindingGetReq{
		BindingType:        req.BindingType,
		BindingClusterUuid: req.BindingClusterUuid,
		BindingProjectId:   req.BindingProjectId,
	})
	if err != nil {
		l.Errorf("获取计费配置绑定失败: %v", err)
		return nil, fmt.Errorf("获取计费配置绑定失败: %v", err)
	}

	// 转换响应数据
	return &types.OnecBillingConfigBindingGetResponse{
		Data: types.OnecBillingConfigBinding{
			Id:                 result.Data.Id,
			BindingType:        result.Data.BindingType,
			BindingClusterUuid: result.Data.BindingClusterUuid,
			BindingProjectId:   result.Data.BindingProjectId,
			PriceConfigId:      result.Data.PriceConfigId,
			BillingStartTime:   result.Data.BillingStartTime,
			LastBillingTime:    result.Data.LastBillingTime,
			CreatedBy:          result.Data.CreatedBy,
			UpdatedBy:          result.Data.UpdatedBy,
			CreatedAt:          result.Data.CreatedAt,
			UpdatedAt:          result.Data.UpdatedAt,
		},
	}, nil
}
