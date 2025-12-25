package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateRetentionPolicyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新保留策略
func NewUpdateRetentionPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateRetentionPolicyLogic {
	return &UpdateRetentionPolicyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateRetentionPolicyLogic) UpdateRetentionPolicy(req *types.UpdateRetentionPolicyRequest) (resp string, err error) {
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

	_, err = l.svcCtx.RepositoryRpc.UpdateRetentionPolicy(l.ctx, &pb.UpdateRetentionPolicyReq{
		RegistryUuid: req.RegistryUuid,
		PolicyId:     req.PolicyId,
		Algorithm:    req.Algorithm,
		Rules:        rules,
		Schedule:     req.Schedule,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("保留策略更新成功: PolicyId=%d", req.PolicyId)
	return "保留策略更新成功", nil
}
