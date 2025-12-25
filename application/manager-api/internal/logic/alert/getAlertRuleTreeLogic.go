package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetAlertRuleTreeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取告警规则树形结构
func NewGetAlertRuleTreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertRuleTreeLogic {
	return &GetAlertRuleTreeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAlertRuleTreeLogic) GetAlertRuleTree(req *types.GetAlertRuleTreeRequest) (resp *types.GetAlertRuleTreeResponse, err error) {
	// 调用RPC服务获取告警规则树形结构
	result, err := l.svcCtx.ManagerRpc.AlertRuleTreeGet(l.ctx, &pb.GetAlertRuleTreeReq{
		FileId: req.FileId,
	})

	if err != nil {
		l.Errorf("获取告警规则树形结构失败: %v", err)
		return nil, fmt.Errorf("获取告警规则树形结构失败: %v", err)
	}

	// 转换树节点
	resp = &types.GetAlertRuleTreeResponse{
		Data: convertTreeNodes(result.Data),
	}

	return resp, nil
}

// 转换树节点递归函数
func convertTreeNodes(pbNodes []*pb.AlertRuleTreeNode) []types.AlertRuleTreeNode {
	if len(pbNodes) == 0 {
		return []types.AlertRuleTreeNode{}
	}

	nodes := make([]types.AlertRuleTreeNode, 0, len(pbNodes))
	for _, pbNode := range pbNodes {
		node := types.AlertRuleTreeNode{
			Id:        pbNode.Id,
			Code:      pbNode.Code,
			Name:      pbNode.Name,
			Type:      pbNode.Type,
			IsEnabled: pbNode.IsEnabled,
			Severity:  pbNode.Severity,
			Children:  convertTreeNodes(pbNode.Children),
		}
		nodes = append(nodes, node)
	}

	return nodes
}
