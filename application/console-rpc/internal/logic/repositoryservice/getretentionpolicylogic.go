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
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	policy, err := client.Retention().GetByProject(in.ProjectName)
	if err != nil {
		return nil, errorx.Msg("获取保留策略失败")
	}

	resp := &pb.GetRetentionPolicyResp{
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

	return resp, nil
}
