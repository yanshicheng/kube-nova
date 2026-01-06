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

type OnecBillingConfigBindingGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOnecBillingConfigBindingGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingConfigBindingGetLogic {
	return &OnecBillingConfigBindingGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// OnecBillingConfigBindingGet 获取计费配置绑定
func (l *OnecBillingConfigBindingGetLogic) OnecBillingConfigBindingGet(in *pb.OnecBillingConfigBindingGetReq) (*pb.OnecBillingConfigBindingGetResp, error) {
	l.Logger.Infof("开始查询计费配置绑定，绑定类型: %s, 集群UUID: %s, 项目ID: %d",
		in.BindingType, in.BindingClusterUuid, in.BindingProjectId)

	// 参数校验
	if in.BindingType == "" {
		l.Logger.Error("绑定类型不能为空")
		return nil, errorx.Msg("绑定类型不能为空")
	}
	if in.BindingClusterUuid == "" {
		l.Logger.Error("绑定集群UUID不能为空")
		return nil, errorx.Msg("绑定集群UUID不能为空")
	}

	// 根据唯一键查询绑定关系
	binding, err := l.svcCtx.OnecBillingConfigBindingModel.FindOneByBindingTypeBindingClusterUuidBindingProjectId(
		l.ctx, in.BindingType, in.BindingClusterUuid, in.BindingProjectId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Infof("计费配置绑定不存在")
			return &pb.OnecBillingConfigBindingGetResp{
				Data: &pb.OnecBillingConfigBinding{},
			}, nil
		}
		l.Logger.Errorf("查询计费配置绑定失败，错误: %v", err)
		return nil, errorx.Msg("计费配置绑定不存在")
	}

	// 检查是否已删除
	if binding.IsDeleted == 1 {
		l.Logger.Error("计费配置绑定已删除")
		return nil, errorx.Msg("计费配置绑定不存在")
	}

	// 转换响应数据
	data := &pb.OnecBillingConfigBinding{
		Id:                 binding.Id,
		BindingType:        binding.BindingType,
		BindingClusterUuid: binding.BindingClusterUuid,
		BindingProjectId:   binding.BindingProjectId,
		PriceConfigId:      binding.PriceConfigId,
		CreatedBy:          binding.CreatedBy,
		UpdatedBy:          binding.UpdatedBy,
		CreatedAt:          binding.CreatedAt.Unix(),
		UpdatedAt:          binding.UpdatedAt.Unix(),
	}
	if binding.BillingStartTime.Valid {
		data.BillingStartTime = binding.BillingStartTime.Time.Unix()
	}
	if binding.LastBillingTime.Valid {
		data.LastBillingTime = binding.LastBillingTime.Time.Unix()
	}

	l.Logger.Infof("查询计费配置绑定成功，绑定ID: %d", binding.Id)
	return &pb.OnecBillingConfigBindingGetResp{
		Data: data,
	}, nil
}
