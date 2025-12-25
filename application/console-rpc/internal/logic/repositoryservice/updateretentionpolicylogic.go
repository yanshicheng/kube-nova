package repositoryservicelogic

import (
	"context"
	"encoding/json"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateRetentionPolicyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateRetentionPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateRetentionPolicyLogic {
	return &UpdateRetentionPolicyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateRetentionPolicyLogic) UpdateRetentionPolicy(in *pb.UpdateRetentionPolicyReq) (*pb.UpdateRetentionPolicyResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	policy := &types.RetentionPolicy{
		Algorithm: in.Algorithm,
	}

	// 转换规则
	for _, pbRule := range in.Rules {
		rule := types.RetentionRule{
			ID:       pbRule.Id,
			Priority: int(pbRule.Priority),
			Disabled: pbRule.Disabled,
			Action:   pbRule.Action,
			Template: pbRule.Template,
			Params:   pbRule.Params,
		}

		// 反序列化 JSON 字符串
		if pbRule.TagSelectors != "" {
			json.Unmarshal([]byte(pbRule.TagSelectors), &rule.TagSelectors)
		}
		if pbRule.ScopeSelectors != "" {
			json.Unmarshal([]byte(pbRule.ScopeSelectors), &rule.ScopeSelectors)
		}

		policy.Rules = append(policy.Rules, rule)
	}

	// 设置触发器
	if in.Schedule != "" {
		policy.Trigger = &types.Trigger{
			Kind: "Schedule",
			Settings: map[string]string{
				"cron": in.Schedule,
			},
		}
	}

	err = client.Retention().Update(in.PolicyId, policy)
	if err != nil {
		return nil, errorx.Msg("更新保留策略失败")
	}

	return &pb.UpdateRetentionPolicyResp{
		Message: "保留策略更新成功",
	}, nil
}
