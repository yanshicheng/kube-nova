package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingPriceConfigGetByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOnecBillingPriceConfigGetByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingPriceConfigGetByIdLogic {
	return &OnecBillingPriceConfigGetByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// OnecBillingPriceConfigGetById 根据ID获取收费配置
func (l *OnecBillingPriceConfigGetByIdLogic) OnecBillingPriceConfigGetById(in *pb.OnecBillingPriceConfigGetByIdReq) (*pb.OnecBillingPriceConfigGetByIdResp, error) {
	l.Logger.Infof("开始查询收费配置，配置ID: %d", in.Id)

	// 参数校验
	if in.Id == 0 {
		l.Logger.Error("配置ID不能为空")
		return nil, errorx.Msg("配置ID不能为空")
	}

	// 查询配置
	config, err := l.svcCtx.OnecBillingPriceConfigModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("查询收费配置失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("收费配置不存在")
	}

	// 检查是否已删除
	if config.IsDeleted == 1 {
		l.Logger.Errorf("收费配置已删除，ID: %d", in.Id)
		return nil, errorx.Msg("收费配置不存在")
	}

	l.Logger.Infof("查询收费配置成功，配置ID: %d", in.Id)
	return &pb.OnecBillingPriceConfigGetByIdResp{
		Data: &pb.OnecBillingPriceConfig{
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
		},
	}, nil
}
