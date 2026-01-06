package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingStatementBatchDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOnecBillingStatementBatchDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingStatementBatchDelLogic {
	return &OnecBillingStatementBatchDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// OnecBillingStatementBatchDel 批量删除账单
func (l *OnecBillingStatementBatchDelLogic) OnecBillingStatementBatchDel(in *pb.OnecBillingStatementBatchDelReq) (*pb.OnecBillingStatementBatchDelResp, error) {
	l.Logger.Infof("开始批量删除账单，截止时间: %d, 集群UUID: %s, 项目ID: %d",
		in.BeforeTime, in.ClusterUuid, in.ProjectId)

	// 参数校验
	if in.BeforeTime == 0 {
		l.Logger.Error("截止时间不能为空")
		return nil, errorx.Msg("截止时间不能为空")
	}

	// 执行批量删除
	deletedCount, err := l.svcCtx.OnecBillingStatementModel.DeleteByCondition(
		l.ctx,
		in.BeforeTime,
		in.ClusterUuid,
		in.ProjectId,
	)
	if err != nil {
		l.Logger.Errorf("批量删除账单失败，错误: %v", err)
		return nil, errorx.Msg("批量删除账单失败")
	}

	l.Logger.Infof("批量删除账单成功，删除数量: %d", deletedCount)
	return &pb.OnecBillingStatementBatchDelResp{
		DeletedCount: deletedCount,
	}, nil
}
