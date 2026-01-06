package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingPriceConfigAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOnecBillingPriceConfigAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingPriceConfigAddLogic {
	return &OnecBillingPriceConfigAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// OnecBillingPriceConfigAdd 新增收费配置
func (l *OnecBillingPriceConfigAddLogic) OnecBillingPriceConfigAdd(in *pb.OnecBillingPriceConfigAddReq) (*pb.OnecBillingPriceConfigAddResp, error) {
	l.Logger.Infof("开始新增收费配置，配置名称: %s", in.ConfigName)

	// 参数校验
	if in.ConfigName == "" {
		l.Logger.Error("配置名称不能为空")
		return nil, errorx.Msg("配置名称不能为空")
	}

	// 检查配置名称是否已存在
	existConfigs, err := l.svcCtx.OnecBillingPriceConfigModel.SearchByName(l.ctx, in.ConfigName)
	if err != nil {
		l.Logger.Errorf("查询配置名称失败，错误: %v", err)
		return nil, errorx.Msg("查询配置信息失败")
	}
	for _, config := range existConfigs {
		if config.ConfigName == in.ConfigName {
			l.Logger.Errorf("配置名称已存在: %s", in.ConfigName)
			return nil, errorx.Msg("配置名称已存在")
		}
	}

	// 构建数据模型
	data := &model.OnecBillingPriceConfig{
		ConfigName:    in.ConfigName,
		Description:   in.Description,
		CpuPrice:      in.CpuPrice,
		MemoryPrice:   in.MemoryPrice,
		StoragePrice:  in.StoragePrice,
		GpuPrice:      in.GpuPrice,
		PodPrice:      in.PodPrice,
		ManagementFee: in.ManagementFee,
		IsSystem:      in.IsSystem,
		CreatedBy:     in.CreatedBy,
		UpdatedBy:     in.CreatedBy,
		IsDeleted:     0,
	}

	// 执行插入
	_, err = l.svcCtx.OnecBillingPriceConfigModel.Insert(l.ctx, data)
	if err != nil {
		l.Logger.Errorf("新增收费配置失败，错误: %v", err)
		return nil, errorx.Msg("新增收费配置失败")
	}

	l.Logger.Infof("新增收费配置成功，配置名称: %s", in.ConfigName)
	return &pb.OnecBillingPriceConfigAddResp{}, nil
}
