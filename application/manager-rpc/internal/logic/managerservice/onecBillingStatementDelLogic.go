package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingStatementDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOnecBillingStatementDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingStatementDelLogic {
	return &OnecBillingStatementDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// OnecBillingStatementDel 删除单条账单
func (l *OnecBillingStatementDelLogic) OnecBillingStatementDel(in *pb.OnecBillingStatementDelReq) (*pb.OnecBillingStatementDelResp, error) {
	l.Logger.Infof("开始删除账单，账单ID: %d", in.Id)

	// 参数校验
	if in.Id == 0 {
		l.Logger.Error("账单ID不能为空")
		return nil, errorx.Msg("账单ID不能为空")
	}

	// 检查账单是否存在
	_, err := l.svcCtx.OnecBillingStatementModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("查询账单失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("账单不存在")
	}

	// 执行软删除
	err = l.svcCtx.OnecBillingStatementModel.SoftDelete(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("删除账单失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("删除账单失败")
	}

	l.Logger.Infof("删除账单成功，账单ID: %d", in.Id)
	return &pb.OnecBillingStatementDelResp{}, nil
}
