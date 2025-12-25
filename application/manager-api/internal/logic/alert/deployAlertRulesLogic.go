package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeployAlertRulesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 将告警规则部署到指定集群
func NewDeployAlertRulesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeployAlertRulesLogic {
	return &DeployAlertRulesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeployAlertRulesLogic) DeployAlertRules(req *types.DeployAlertRulesRequest) (resp *types.DeployAlertRulesResponse, err error) {
	// 调用RPC服务部署告警规则
	result, err := l.svcCtx.ManagerRpc.AlertRulesDeploy(l.ctx, &pb.DeployAlertRulesReq{
		FileId:      req.FileId,
		ClusterUuid: req.ClusterUuid,
		Namespace:   req.Namespace,
		GroupIds:    req.GroupIds,
		RuleIds:     req.RuleIds,
	})

	if err != nil {
		l.Errorf("部署告警规则失败: %v", err)
		return nil, fmt.Errorf("部署告警规则失败: %v", err)
	}

	resp = &types.DeployAlertRulesResponse{
		YamlStr: result.YamlStr,
	}

	return resp, nil
}
