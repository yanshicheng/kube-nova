package managerservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingPriceConfigSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOnecBillingPriceConfigSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingPriceConfigSearchLogic {
	return &OnecBillingPriceConfigSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// OnecBillingPriceConfigSearch 搜索收费配置
func (l *OnecBillingPriceConfigSearchLogic) OnecBillingPriceConfigSearch(in *pb.OnecBillingPriceConfigSearchReq) (*pb.OnecBillingPriceConfigSearchResp, error) {
	l.Logger.Infof("开始搜索收费配置，配置名称: %s", in.ConfigName)

	// 执行搜索
	configs, err := l.svcCtx.OnecBillingPriceConfigModel.SearchByName(l.ctx, in.ConfigName)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Logger.Infof("搜索收费配置成功，返回数量: 0")
			return &pb.OnecBillingPriceConfigSearchResp{}, nil
		}
		l.Logger.Errorf("搜索收费配置失败，错误: %v", err)
		return nil, errorx.Msg("搜索收费配置失败")
	}

	// 转换响应数据
	var data []*pb.OnecBillingPriceConfig
	for _, config := range configs {
		data = append(data, &pb.OnecBillingPriceConfig{
			Id:            config.Id,
			ConfigName:    config.ConfigName,
			Description:   config.Description,
			CpuPrice:      config.CpuPrice,
			MemoryPrice:   config.MemoryPrice,
			StoragePrice:  config.StoragePrice,
			GpuPrice:      config.GpuPrice,
			PodPrice:      config.PodPrice,
			ManagementFee: config.ManagementFee,
			IsSystem:      config.IsSystem,
			CreatedBy:     config.CreatedBy,
			UpdatedBy:     config.UpdatedBy,
			CreatedAt:     config.CreatedAt.Unix(),
			UpdatedAt:     config.UpdatedAt.Unix(),
		})
	}

	l.Logger.Infof("搜索收费配置成功，返回数量: %d", len(data))
	return &pb.OnecBillingPriceConfigSearchResp{
		Data: data,
	}, nil
}
