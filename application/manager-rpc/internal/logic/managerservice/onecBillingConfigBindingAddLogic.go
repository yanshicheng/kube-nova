package managerservicelogic

import (
	"context"
	"database/sql"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingConfigBindingAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOnecBillingConfigBindingAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingConfigBindingAddLogic {
	return &OnecBillingConfigBindingAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// OnecBillingConfigBindingAdd 新增计费配置绑定
func (l *OnecBillingConfigBindingAddLogic) OnecBillingConfigBindingAdd(in *pb.OnecBillingConfigBindingAddReq) (*pb.OnecBillingConfigBindingAddResp, error) {
	l.Logger.Infof("开始新增计费配置绑定，集群UUID: %s, 项目ID: %d", in.BindingClusterUuid, in.BindingProjectId)

	// 参数校验
	if in.BindingType == "" {
		l.Logger.Error("绑定类型不能为空")
		return nil, errorx.Msg("绑定类型不能为空")
	}
	if in.BindingClusterUuid == "" {
		l.Logger.Error("绑定集群UUID不能为空")
		return nil, errorx.Msg("绑定集群UUID不能为空")
	}
	if in.PriceConfigId == 0 {
		l.Logger.Error("价格配置ID不能为空")
		return nil, errorx.Msg("价格配置ID不能为空")
	}

	// 检查价格配置是否存在
	_, err := l.svcCtx.OnecBillingPriceConfigModel.FindOne(l.ctx, in.PriceConfigId)
	if err != nil {
		l.Logger.Errorf("价格配置不存在，配置ID: %d, 错误: %v", in.PriceConfigId, err)
		return nil, errorx.Msg("价格配置不存在")
	}

	// 检查绑定是否已存在
	_, err = l.svcCtx.OnecBillingConfigBindingModel.FindOneByBindingTypeBindingClusterUuidBindingProjectId(
		l.ctx, in.BindingType, in.BindingClusterUuid, in.BindingProjectId)
	if err == nil {
		l.Logger.Errorf("绑定关系已存在，类型: %s, 集群: %s, 项目: %d", in.BindingType, in.BindingClusterUuid, in.BindingProjectId)
		return nil, errorx.Msg("该绑定关系已存在")
	}

	// 处理计费开始时间
	var billingStartTime sql.NullTime
	if in.BillingStartTime > 0 {
		billingStartTime = sql.NullTime{
			Time:  time.Unix(in.BillingStartTime, 0),
			Valid: true,
		}
	}

	// 构建数据模型
	data := &model.OnecBillingConfigBinding{
		BindingType:        in.BindingType,
		BindingClusterUuid: in.BindingClusterUuid,
		BindingProjectId:   in.BindingProjectId,
		PriceConfigId:      in.PriceConfigId,
		BillingStartTime:   billingStartTime,
		CreatedBy:          in.CreatedBy,
		UpdatedBy:          in.CreatedBy,
		IsDeleted:          0,
	}

	// 执行插入
	_, err = l.svcCtx.OnecBillingConfigBindingModel.Insert(l.ctx, data)
	if err != nil {
		l.Logger.Errorf("新增计费配置绑定失败，错误: %v", err)
		return nil, errorx.Msg("新增计费配置绑定失败")
	}

	l.Logger.Infof("新增计费配置绑定成功，集群UUID: %s, 项目ID: %d", in.BindingClusterUuid, in.BindingProjectId)
	return &pb.OnecBillingConfigBindingAddResp{}, nil
}
