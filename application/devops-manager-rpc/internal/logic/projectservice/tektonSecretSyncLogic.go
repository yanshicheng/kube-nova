package projectservicelogic

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	devopstypes "github.com/yanshicheng/kube-nova/common/devops/types"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonSecretSyncLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonSecretSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonSecretSyncLogic {
	return &TektonSecretSyncLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonSecretSyncLogic) TektonSecretSync(in *pb.SyncTektonSecretReq) (*pb.EmptyResp, error) {
	sourceRef, err := resolveTektonSecretBinding(l.ctx, l.svcCtx, in.ProjectId, in.SourceBindingId, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("解析源 Tekton Secret 渠道失败: %v", err)
		return nil, err
	}
	source, err := sourceRef.Client.GetProjectSecret(l.ctx, sourceRef.BindingConfig.Namespace, in.Name, in.ProjectId, sourceRef.Binding.ID.Hex())
	if err != nil {
		l.Errorf("读取源 Tekton Secret 失败: %v", err)
		return nil, tektonSecretClientError(err)
	}
	targetIDs := uniqueNonEmptyStrings(in.TargetBindingIds)
	if len(targetIDs) == 0 {
		return nil, errorx.Msg("请选择目标 Tekton 渠道")
	}
	validTargetCount := 0
	var failed []string
	for _, targetID := range targetIDs {
		if targetID == sourceRef.Binding.ID.Hex() {
			continue
		}
		validTargetCount++
		targetRef, err := resolveTektonSecretBinding(l.ctx, l.svcCtx, in.ProjectId, targetID, in.CurrentUserId, in.CurrentRoles)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %s", targetID, devopstypes.TrimMessage(err.Error())))
			continue
		}
		secret := copyKubernetesSecretForApply(source)
		secret.Labels["kube-nova.io/devops-project-id"] = in.ProjectId
		secret.Labels["kube-nova.io/devops-binding-id"] = targetRef.Binding.ID.Hex()
		secret.Annotations["kube-nova.io/updated-by"] = strings.TrimSpace(in.Operator)
		if err := targetRef.Client.ApplyProjectSecret(l.ctx, targetRef.BindingConfig.Namespace, secret); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %s", targetRef.Channel.Name, devopstypes.TrimMessage(err.Error())))
		}
	}
	if validTargetCount == 0 {
		return nil, errorx.Msg("请选择源渠道之外的目标 Tekton 渠道")
	}
	if len(failed) > 0 {
		l.Errorf("同步项目 Tekton Secret 部分失败: %s", strings.Join(failed, "、"))
		return nil, errorx.Msg("部分 Tekton Secret 同步失败: " + strings.Join(failed, "、"))
	}

	return &pb.EmptyResp{}, nil
}

func uniqueNonEmptyStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}
