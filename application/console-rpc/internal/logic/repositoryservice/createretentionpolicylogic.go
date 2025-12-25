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

type CreateRetentionPolicyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateRetentionPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateRetentionPolicyLogic {
	return &CreateRetentionPolicyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateRetentionPolicyLogic) CreateRetentionPolicy(in *pb.CreateRetentionPolicyReq) (*pb.CreateRetentionPolicyResp, error) {
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
			Priority: int(pbRule.Priority),
			Disabled: pbRule.Disabled,
			Action:   pbRule.Action,
			Template: pbRule.Template,
			Params:   pbRule.Params,
		}

		// 反序列化 JSON 字符串为对象
		if pbRule.TagSelectors != "" {
			json.Unmarshal([]byte(pbRule.TagSelectors), &rule.TagSelectors)
		}
		if pbRule.ScopeSelectors != "" {
			json.Unmarshal([]byte(pbRule.ScopeSelectors), &rule.ScopeSelectors)
		}

		policy.Rules = append(policy.Rules, rule)
	}

	// 设置触发器（如果提供了 schedule）
	if in.Schedule != "" {
		policy.Trigger = &types.Trigger{
			Kind: "Schedule",
			Settings: map[string]string{
				"cron": in.Schedule,
			},
		}
	}

	policyID, err := client.Retention().Create(in.ProjectName, policy)
	if err != nil {
		return nil, errorx.Msg("创建保留策略失败")
	}

	return &pb.CreateRetentionPolicyResp{
		Id:      policyID,
		Message: "保留策略创建成功",
	}, nil
}
