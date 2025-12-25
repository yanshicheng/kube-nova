package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeployAllAlertRulesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewDeployAllAlertRulesLogic 部署全部告警规则到指定集群
func NewDeployAllAlertRulesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeployAllAlertRulesLogic {
	return &DeployAllAlertRulesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeployAllAlertRulesLogic) DeployAllAlertRules(req *types.DeployAllAlertRulesRequest) (resp *types.DeployAllAlertRulesResponse, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用RPC服务部署全部告警规则
	result, err := l.svcCtx.ManagerRpc.AlertRulesDeployAll(l.ctx, &pb.DeployAllAlertRulesReq{
		ClusterUuid: req.ClusterUuid,
		Namespace:   req.Namespace,
		CreatedBy:   username,
	})

	if err != nil {
		l.Errorf("部署全部告警规则失败: %v", err)
		return nil, fmt.Errorf("部署全部告警规则失败: %v", err)
	}

	resp = &types.DeployAllAlertRulesResponse{
		FileCount:  result.FileCount,
		GroupCount: result.GroupCount,
		RuleCount:  result.RuleCount,
	}

	return resp, nil
}
