package repositoryservicelogic

import (
	"context"
	"encoding/json"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRetentionPolicyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRetentionPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRetentionPolicyLogic {
	return &GetRetentionPolicyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ============ 保留策略管理 ============
func (l *GetRetentionPolicyLogic) GetRetentionPolicy(in *pb.GetRetentionPolicyReq) (*pb.GetRetentionPolicyResp, error) {
	l.Infof("获取保留策略: registryUuid=%s, projectName=%s", in.RegistryUuid, in.ProjectName)

	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		l.Errorf("获取仓库客户端失败: %v", err)
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	policy, err := client.Retention().GetByProject(in.ProjectName)
	if err != nil {
		l.Errorf("获取保留策略失败: %v", err)
		return nil, errorx.Msg("获取保留策略失败")
	}

	// 返回 Exists=false，而不是返回 nil
	if policy == nil {
		l.Infof("项目 %s 没有配置保留策略", in.ProjectName)
		return &pb.GetRetentionPolicyResp{
			Exists: false, // 明确标识不存在
			Data:   nil,
		}, nil
	}

	// 策略存在，构造响应
	resp := &pb.GetRetentionPolicyResp{
		Exists: true, // 明确标识存在
		Data: &pb.RetentionPolicy{
			Id:         policy.ID,
			Algorithm:  policy.Algorithm,
			CreateTime: policy.CreateTime.Unix(),
			UpdateTime: policy.UpdateTime.Unix(),
		},
	}

	// 转换规则
	for _, rule := range policy.Rules {
		// 将 TagSelectors 序列化为 JSON 字符串
		tagSelectorsJSON, _ := json.Marshal(rule.TagSelectors)
		scopeSelectorsJSON, _ := json.Marshal(rule.ScopeSelectors)

		pbRule := &pb.RetentionRule{
			Id:             rule.ID,
			Priority:       int32(rule.Priority),
			Disabled:       rule.Disabled,
			Action:         rule.Action,
			Template:       rule.Template,
			Params:         rule.Params,
			TagSelectors:   string(tagSelectorsJSON),
			ScopeSelectors: string(scopeSelectorsJSON),
		}

		resp.Data.Rules = append(resp.Data.Rules, pbRule)
	}

	// 将 Trigger 序列化为 JSON 字符串
	if policy.Trigger != nil {
		triggerJSON, _ := json.Marshal(policy.Trigger)
		resp.Data.Trigger = string(triggerJSON)
	}

	// 将 Scope 序列化为 JSON 字符串
	if policy.Scope != nil {
		scopeJSON, _ := json.Marshal(policy.Scope)
		resp.Data.Scope = string(scopeJSON)
	}

	l.Infof("获取保留策略成功: policyId=%d, rulesCount=%d", policy.ID, len(policy.Rules))
	return resp, nil
}
