package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingConfigBindingUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOnecBillingConfigBindingUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingConfigBindingUpdateLogic {
	return &OnecBillingConfigBindingUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// OnecBillingConfigBindingUpdate 更新计费配置绑定
func (l *OnecBillingConfigBindingUpdateLogic) OnecBillingConfigBindingUpdate(in *pb.OnecBillingConfigBindingUpdateReq) (*pb.OnecBillingConfigBindingUpdateResp, error) {
	l.Logger.Infof("开始更新计费配置绑定，绑定ID: %d", in.Id)

	// 参数校验
	if in.Id == 0 {
		l.Logger.Error("绑定ID不能为空")
		return nil, errorx.Msg("绑定ID不能为空")
	}
	if in.PriceConfigId == 0 {
		l.Logger.Error("价格配置ID不能为空")
		return nil, errorx.Msg("价格配置ID不能为空")
	}

	// 检查绑定是否存在
	existBinding, err := l.svcCtx.OnecBillingConfigBindingModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("查询绑定关系失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("绑定关系不存在")
	}

	// 检查价格配置是否存在
	_, err = l.svcCtx.OnecBillingPriceConfigModel.FindOne(l.ctx, in.PriceConfigId)
	if err != nil {
		l.Logger.Errorf("价格配置不存在，配置ID: %d, 错误: %v", in.PriceConfigId, err)
		return nil, errorx.Msg("价格配置不存在")
	}

	// 构建更新数据
	data := &model.OnecBillingConfigBinding{
		Id:                 in.Id,
		BindingType:        existBinding.BindingType,
		BindingClusterUuid: in.BindingClusterUuid,
		BindingProjectId:   in.BindingProjectId,
		PriceConfigId:      in.PriceConfigId,
		BillingStartTime:   existBinding.BillingStartTime,
		LastBillingTime:    existBinding.LastBillingTime,
		CreatedBy:          existBinding.CreatedBy,
		UpdatedBy:          in.UpdatedBy,
		IsDeleted:          existBinding.IsDeleted,
	}

	// 如果传入的集群UUID或项目ID为空，则使用原值
	if in.BindingClusterUuid == "" {
		data.BindingClusterUuid = existBinding.BindingClusterUuid
	}
	if in.BindingProjectId == 0 {
		data.BindingProjectId = existBinding.BindingProjectId
	}

	// 执行更新
	err = l.svcCtx.OnecBillingConfigBindingModel.Update(l.ctx, data)
	if err != nil {
		l.Logger.Errorf("更新计费配置绑定失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("更新计费配置绑定失败")
	}

	l.Logger.Infof("更新计费配置绑定成功，绑定ID: %d", in.Id)
	return &pb.OnecBillingConfigBindingUpdateResp{}, nil
}
