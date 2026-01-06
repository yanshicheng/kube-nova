package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingConfigBindingDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOnecBillingConfigBindingDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingConfigBindingDelLogic {
	return &OnecBillingConfigBindingDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// OnecBillingConfigBindingDel 删除计费配置绑定
func (l *OnecBillingConfigBindingDelLogic) OnecBillingConfigBindingDel(in *pb.OnecBillingConfigBindingDelReq) (*pb.OnecBillingConfigBindingDelResp, error) {
	l.Logger.Infof("开始删除计费配置绑定，绑定ID: %d", in.Id)

	// 参数校验
	if in.Id == 0 {
		l.Logger.Error("绑定ID不能为空")
		return nil, errorx.Msg("绑定ID不能为空")
	}

	// 检查绑定是否存在
	_, err := l.svcCtx.OnecBillingConfigBindingModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("查询绑定关系失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("绑定关系不存在")
	}

	// 执行删除
	err = l.svcCtx.OnecBillingConfigBindingModel.Delete(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("删除计费配置绑定失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("删除计费配置绑定失败")
	}

	l.Logger.Infof("删除计费配置绑定成功，绑定ID: %d", in.Id)
	return &pb.OnecBillingConfigBindingDelResp{}, nil
}
