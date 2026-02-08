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
	l.Infof("更新保留策略: registryUuid=%s, projectName=%s, policyId=%d, algorithm=%s, rulesCount=%d",
		in.RegistryUuid, in.ProjectName, in.PolicyId, in.Algorithm, len(in.Rules))

	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		l.Errorf("获取仓库客户端失败: %v", err)
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	// 构造策略对象
	policy := &types.RetentionPolicy{
		Algorithm: in.Algorithm,
	}

	// 设置默认算法
	if policy.Algorithm == "" {
		policy.Algorithm = "or"
	}

	// 转换规则
	for i, pbRule := range in.Rules {
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
			if err := json.Unmarshal([]byte(pbRule.TagSelectors), &rule.TagSelectors); err != nil {
				l.Errorf("解析 TagSelectors 失败 (rule %d): %v", i, err)
				return nil, errorx.Msg("解析标签选择器失败")
			}
		}
		if pbRule.ScopeSelectors != "" {
			if err := json.Unmarshal([]byte(pbRule.ScopeSelectors), &rule.ScopeSelectors); err != nil {
				l.Errorf("解析 ScopeSelectors 失败 (rule %d): %v", i, err)
				return nil, errorx.Msg("解析范围选择器失败")
			}
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

	if in.PolicyId <= 0 {
		l.Infof("PolicyId 为空，将创建新策略: projectName=%s", in.ProjectName)

		if in.ProjectName == "" {
			l.Errorf("创建策略失败: projectName 不能为空")
			return nil, errorx.Msg("项目名称不能为空")
		}

		policyID, err := client.Retention().Create(in.ProjectName, policy)
		if err != nil {
			l.Errorf("创建保留策略失败: %v", err)
			return nil, errorx.Msg("创建保留策略失败")
		}

		l.Infof("创建保留策略成功: policyId=%d", policyID)
		return &pb.UpdateRetentionPolicyResp{
			Id:      policyID,
			Message: "保留策略创建成功",
		}, nil
	}

	// PolicyId > 0，执行更新操作
	err = client.Retention().Update(in.PolicyId, policy)
	if err != nil {
		l.Errorf("更新保留策略失败: %v", err)
		return nil, errorx.Msg("更新保留策略失败")
	}

	l.Infof("更新保留策略成功: policyId=%d", in.PolicyId)
	return &pb.UpdateRetentionPolicyResp{
		Id:      in.PolicyId,
		Message: "保留策略更新成功",
	}, nil
}
