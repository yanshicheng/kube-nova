package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingPriceConfigUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOnecBillingPriceConfigUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingPriceConfigUpdateLogic {
	return &OnecBillingPriceConfigUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// OnecBillingPriceConfigUpdate 更新收费配置
func (l *OnecBillingPriceConfigUpdateLogic) OnecBillingPriceConfigUpdate(in *pb.OnecBillingPriceConfigUpdateReq) (*pb.OnecBillingPriceConfigUpdateResp, error) {
	l.Logger.Infof("开始更新收费配置，配置ID: %d", in.Id)

	// 参数校验
	if in.Id == 0 {
		l.Logger.Error("配置ID不能为空")
		return nil, errorx.Msg("配置ID不能为空")
	}
	if in.ConfigName == "" {
		l.Logger.Error("配置名称不能为空")
		return nil, errorx.Msg("配置名称不能为空")
	}

	// 检查配置是否存在
	existConfig, err := l.svcCtx.OnecBillingPriceConfigModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("查询收费配置失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("收费配置不存在")
	}

	// 系统内置配置不允许修改关键字段
	if existConfig.IsSystem == 1 {
		l.Logger.Errorf("系统内置配置不允许修改，ID: %d", in.Id)
		return nil, errorx.Msg("系统内置配置不允许修改")
	}

	// 检查配置名称是否重复
	existConfigs, err := l.svcCtx.OnecBillingPriceConfigModel.SearchByName(l.ctx, in.ConfigName)
	if err != nil {
		l.Logger.Errorf("查询配置名称失败，错误: %v", err)
		return nil, errorx.Msg("查询配置信息失败")
	}
	for _, config := range existConfigs {
		if config.ConfigName == in.ConfigName && config.Id != in.Id {
			l.Logger.Errorf("配置名称已存在: %s", in.ConfigName)
			return nil, errorx.Msg("配置名称已存在")
		}
	}

	// 构建更新数据
	data := &model.OnecBillingPriceConfig{
		Id:            in.Id,
		ConfigName:    in.ConfigName,
		Description:   in.Description,
		CpuPrice:      in.CpuPrice,
		MemoryPrice:   in.MemoryPrice,
		StoragePrice:  in.StoragePrice,
		GpuPrice:      in.GpuPrice,
		PodPrice:      in.PodPrice,
		ManagementFee: in.ManagementFee,
		IsSystem:      existConfig.IsSystem,
		CreatedBy:     existConfig.CreatedBy,
		UpdatedBy:     in.UpdatedBy,
		IsDeleted:     existConfig.IsDeleted,
	}

	// 执行更新
	err = l.svcCtx.OnecBillingPriceConfigModel.Update(l.ctx, data)
	if err != nil {
		l.Logger.Errorf("更新收费配置失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("更新收费配置失败")
	}

	l.Logger.Infof("更新收费配置成功，配置ID: %d", in.Id)
	return &pb.OnecBillingPriceConfigUpdateResp{}, nil
}
