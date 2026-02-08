package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateRetentionPolicyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建保留策略
func NewCreateRetentionPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateRetentionPolicyLogic {
	return &CreateRetentionPolicyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateRetentionPolicyLogic) CreateRetentionPolicy(req *types.CreateRetentionPolicyRequest) (resp *types.CreateRetentionPolicyResponse, err error) {
	// 转换规则
	var rules []*pb.RetentionRule
	for _, rule := range req.Rules {
		rules = append(rules, &pb.RetentionRule{
			Id:             rule.Id,
			Priority:       rule.Priority,
			Disabled:       rule.Disabled,
			Action:         rule.Action,
			Template:       rule.Template,
			Params:         rule.Params,
			TagSelectors:   rule.TagSelectors,
			ScopeSelectors: rule.ScopeSelectors,
		})
	}

	rpcResp, err := l.svcCtx.RepositoryRpc.CreateRetentionPolicy(l.ctx, &pb.CreateRetentionPolicyReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
		Algorithm:    req.Algorithm,
		Rules:        rules,
		Schedule:     req.Schedule,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	l.Infof("保留策略创建成功: PolicyId=%d", rpcResp.Id)
	return &types.CreateRetentionPolicyResponse{
		Id:      rpcResp.Id,
		Message: "保留策略创建成功",
	}, nil
}
