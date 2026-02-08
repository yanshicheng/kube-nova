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

func (l *UpdateRetentionPolicyLogic) UpdateRetentionPolicy(req *types.UpdateRetentionPolicyRequest) (resp *types.UpdateRetentionPolicyResponse, err error) {
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

	if req.PolicyId <= 0 {
		l.Infof("PolicyId 为空，将创建新策略: project=%s", req.ProjectName)

		// 调用创建接口
		createResp, err := l.svcCtx.RepositoryRpc.CreateRetentionPolicy(l.ctx, &pb.CreateRetentionPolicyReq{
			RegistryUuid: req.RegistryUuid,
			ProjectName:  req.ProjectName,
			Algorithm:    req.Algorithm,
			Rules:        rules,
			Schedule:     req.Schedule,
		})
		if err != nil {
			l.Errorf("RPC创建调用失败: %v", err)
			return nil, err
		}

		l.Infof("保留策略创建成功: PolicyId=%d", createResp.Id)
		return &types.UpdateRetentionPolicyResponse{
			Id:      createResp.Id,
			Message: "保留策略创建成功",
		}, nil
	}

	// PolicyId > 0，执行更新操作
	updateResp, err := l.svcCtx.RepositoryRpc.UpdateRetentionPolicy(l.ctx, &pb.UpdateRetentionPolicyReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
		PolicyId:     req.PolicyId,
		Algorithm:    req.Algorithm,
		Rules:        rules,
		Schedule:     req.Schedule,
	})
	if err != nil {
		l.Errorf("RPC更新调用失败: %v", err)
		return nil, err
	}

	l.Infof("保留策略更新成功: PolicyId=%d", req.PolicyId)
	return &types.UpdateRetentionPolicyResponse{
		Id:      updateResp.Id,
		Message: "保留策略更新成功",
	}, nil
}
