package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetRetentionPolicyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取项目保留策略
func NewGetRetentionPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRetentionPolicyLogic {
	return &GetRetentionPolicyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetRetentionPolicyLogic) GetRetentionPolicy(req *types.GetRetentionPolicyRequest) (resp *types.GetRetentionPolicyResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.GetRetentionPolicy(l.ctx, &pb.GetRetentionPolicyReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	// 如果不存在策略，返回 Exists=false, Data=nil
	if rpcResp == nil || !rpcResp.Exists || rpcResp.Data == nil {
		l.Infof("项目 %s 没有配置保留策略", req.ProjectName)
		return &types.GetRetentionPolicyResponse{
			Exists: false,
			Data:   nil, // 明确返回 nil
		}, nil
	}

	// 转换规则
	var rules []types.RetentionRule
	for _, rule := range rpcResp.Data.Rules {
		rules = append(rules, types.RetentionRule{
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

	l.Infof("保留策略查询成功: PolicyId=%d", rpcResp.Data.Id)
	return &types.GetRetentionPolicyResponse{
		Exists: true,
		Data: &types.RetentionPolicy{
			Id:         rpcResp.Data.Id,
			Algorithm:  rpcResp.Data.Algorithm,
			Rules:      rules,
			Trigger:    rpcResp.Data.Trigger,
			Scope:      rpcResp.Data.Scope,
			CreateTime: rpcResp.Data.CreateTime,
			UpdateTime: rpcResp.Data.UpdateTime,
		},
	}, nil
}
