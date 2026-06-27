package projectservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type TektonResourceDescribeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTektonResourceDescribeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TektonResourceDescribeLogic {
	return &TektonResourceDescribeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TektonResourceDescribeLogic) TektonResourceDescribe(in *pb.DescribeTektonResourceReq) (*pb.DescribeTektonResourceResp, error) {
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
	switch resourceType {
	case "configMap":
		content, err = ref.Client.DescribeConfigMap(l.ctx, ref.BindingConfig.Namespace, name)
	case "pvc":
		content, err = ref.Client.DescribePersistentVolumeClaim(l.ctx, ref.BindingConfig.Namespace, name)
	case "serviceAccount":
		content, err = ref.Client.DescribeServiceAccount(l.ctx, ref.BindingConfig.Namespace, name)
	case "secret":
		content, err = ref.Client.DescribeProjectSecret(l.ctx, ref.BindingConfig.Namespace, name, in.ProjectId, ref.Binding.ID.Hex())
	case "pod":
		content, err = ref.Client.DescribePod(l.ctx, ref.BindingConfig.Namespace, name)
	case "pipeline":
		content, err = ref.Client.DescribePipeline(l.ctx, ref.BindingConfig.Namespace, name)
	case "pipelineRun":
		content, err = ref.Client.DescribePipelineRun(l.ctx, ref.BindingConfig.Namespace, name)
	default:
		return nil, errorx.Msg("Tekton 资源类型不支持")
	}
	if err != nil {
		l.Errorf("获取 Tekton 资源 Describe 失败: type=%s name=%s err=%v", resourceType, name, err)
		return nil, err
	}
	return &pb.DescribeTektonResourceResp{Content: content}, nil
}
