package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingPriceConfigDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOnecBillingPriceConfigDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingPriceConfigDelLogic {
	return &OnecBillingPriceConfigDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// OnecBillingPriceConfigDel 删除收费配置
func (l *OnecBillingPriceConfigDelLogic) OnecBillingPriceConfigDel(in *pb.OnecBillingPriceConfigDelReq) (*pb.OnecBillingPriceConfigDelResp, error) {
	l.Logger.Infof("开始删除收费配置，配置ID: %d", in.Id)

	// 参数校验
	if in.Id == 0 {
		l.Logger.Error("配置ID不能为空")
		return nil, errorx.Msg("配置ID不能为空")
	}

	// 检查配置是否存在
	existConfig, err := l.svcCtx.OnecBillingPriceConfigModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("查询收费配置失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("收费配置不存在")
	}

	// 系统内置配置不允许删除
	if existConfig.IsSystem == 1 {
		l.Logger.Errorf("系统内置配置不允许删除，ID: %d", in.Id)
		return nil, errorx.Msg("系统内置配置不允许删除")
	}

	// 检查是否有绑定引用
	bindings, err := l.svcCtx.OnecBillingConfigBindingModel.FindByPriceConfigId(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("查询配置绑定关系失败，配置ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("查询配置绑定关系失败")
	}
	if len(bindings) > 0 {
		l.Logger.Errorf("收费配置存在绑定引用，无法删除，配置ID: %d, 绑定数量: %d", in.Id, len(bindings))
		return nil, errorx.Msg("该收费配置已被使用，请先解除绑定关系")
	}

	// 执行删除
	err = l.svcCtx.OnecBillingPriceConfigModel.Delete(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("删除收费配置失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("删除收费配置失败")
	}

	l.Logger.Infof("删除收费配置成功，配置ID: %d", in.Id)
	return &pb.OnecBillingPriceConfigDelResp{}, nil
}
