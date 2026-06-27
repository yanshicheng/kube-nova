package projectservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonResourceActionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonResourceActionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonResourceActionLogic {
	return &TektonResourceActionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonResourceActionLogic) TektonResourceAction(in *pb.ActionTektonResourceReq) (*pb.EmptyResp, error) {
	ref, err := resolveTektonSecretBinding(l.ctx, l.svcCtx, in.ProjectId, in.BindingId, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("解析 Tekton 资源绑定失败: %v", err)
		return nil, err
	}
	resourceType := normalizeTektonResourceType(in.ResourceType)
	name := strings.TrimSpace(in.Name)
	action := strings.TrimSpace(in.Action)
	if name == "" {
		return nil, errorx.Msg("资源名称不能为空")
	}
	switch {
	case resourceType == "pipelineRun" && action == "cancel":
		if err := ref.Client.CancelPipelineRun(l.ctx, ref.BindingConfig.Namespace, name); err != nil {
			l.Errorf("取消 Tekton PipelineRun 失败: %v", err)
			return nil, err
		}
		return &pb.EmptyResp{}, nil
	default:
		return nil, errorx.Msg("Tekton 资源动作不支持")
	}
}
