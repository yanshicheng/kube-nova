// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package billing

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type OnecBillingStatementGenerateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 立即生成账单
func NewOnecBillingStatementGenerateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OnecBillingStatementGenerateLogic {
	return &OnecBillingStatementGenerateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OnecBillingStatementGenerateLogic) OnecBillingStatementGenerate(req *types.OnecBillingStatementGenerateRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用RPC服务
	_, err = l.svcCtx.ManagerRpc.OnecBillingStatementGenerate(l.ctx, &pb.OnecBillingStatementGenerateReq{
		ClusterUuid: req.ClusterUuid,
		ProjectId:   req.ProjectId,
		UserName:    username,
	})
	if err != nil {
		l.Errorf("生成账单失败: %v", err)
		return "生成账单失败", err
	}

	return "生成账单成功", nil
}
