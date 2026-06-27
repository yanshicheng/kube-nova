package projectservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonResourceLogLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonResourceLogLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonResourceLogLogic {
	return &TektonResourceLogLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonResourceLogLogic) TektonResourceLog(in *pb.LogTektonResourceReq) (*pb.LogTektonResourceResp, error) {
	ref, err := resolveTektonSecretBinding(l.ctx, l.svcCtx, in.ProjectId, in.BindingId, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("解析 Tekton 资源绑定失败: %v", err)
		return nil, err
	}
	resourceType := normalizeTektonResourceType(in.ResourceType)
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, errorx.Msg("资源名称不能为空")
	}
	var content string
	var containers []string
	switch resourceType {
	case "pod":
		content, containers, err = ref.Client.GetPodLog(l.ctx, ref.BindingConfig.Namespace, name, in.Container, in.TailLines)
	case "pipelineRun":
		content, containers, err = ref.Client.GetPipelineRunLog(l.ctx, ref.BindingConfig.Namespace, name, in.Container, in.TailLines)
	default:
		return nil, errorx.Msg("Tekton 资源类型不支持日志")
	}
	if err != nil {
		l.Errorf("获取 Tekton 资源日志失败: type=%s name=%s err=%v", resourceType, name, err)
		return nil, err
	}
	return &pb.LogTektonResourceResp{Content: content, Containers: containers}, nil
}
