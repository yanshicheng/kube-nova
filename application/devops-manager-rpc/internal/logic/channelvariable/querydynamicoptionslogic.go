package logic

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"

	"github.com/zeromicro/go-zero/core/logx"
)

type QueryDynamicOptionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewQueryDynamicOptionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QueryDynamicOptionsLogic {
	return &QueryDynamicOptionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// QueryDynamicOptions 查询动态选项
func (l *QueryDynamicOptionsLogic) QueryDynamicOptions(in *pb.QueryDynamicOptionsReq) (*pb.QueryDynamicOptionsResp, error) {
	// 1. 获取渠道实例（MongoDB使用string ID）
	channelID := fmt.Sprintf("%d", in.ChannelInstanceId)
	instance, err := l.svcCtx.ChannelModel.FindOne(l.ctx, channelID)
	if err != nil {
		l.Errorf("查询渠道实例失败: %v", err)
		return nil, fmt.Errorf("渠道实例不存在")
	}

	// 2. 获取渠道类型
	channelType, err := l.svcCtx.ChannelTypeModel.FindOneByCode(l.ctx, instance.ChannelType)
	if err != nil {
		l.Errorf("查询渠道类型失败: %v", err)
		return nil, fmt.Errorf("渠道类型不存在")
	}

	// 3. 获取Provider（使用临时映射）
	providerType := getProviderTypeByChannelType(channelType.Code)
	provider := channelvars.GetProviderRegistry().Get(providerType)
	if provider == nil {
		l.Errorf("Provider不存在: %s", providerType)
		return nil, fmt.Errorf("Provider不存在: %s", providerType)
	}

	// 4. 调用Provider查询选项
	req := &channelvars.QueryOptionsRequest{
		ChannelInstanceID: in.ChannelInstanceId,
		ProviderKey:       in.ProviderKey,
		ParentValues:      in.ParentValues,
		ProjectID:         in.ProjectId,
	}

	resp, err := provider.QueryOptions(l.ctx, req)
	if err != nil {
		l.Errorf("查询动态选项失败: %v", err)
		return nil, err
	}

	// 5. 转换为pb格式
	pbOptions := make([]*pb.VariableOption, len(resp.Options))
	for i, opt := range resp.Options {
		pbOptions[i] = convertOptionToPb(opt)
	}

	return &pb.QueryDynamicOptionsResp{
		Options: pbOptions,
	}, nil
}

// getProviderTypeByChannelType 根据渠道类型获取Provider类型（临时映射）
func getProviderTypeByChannelType(channelTypeCode string) string {
	mapping := map[string]string{
		// Git 代码仓库
		"gitlab": "git",
		"github": "git",
		"gitee":  "git",
		"svn":    "svn",
		// 镜像仓库
		"harbor":          "harbor",
		"registry":        "registry",
		"aliyun_registry": "registry",
		// 制品仓库
		"nexus": "nexus",
		"jfrog": "jfrog",
		// 代码扫描
		"sonarqube": "sonarqube",
		"spotbugs":  "spotbugs",
		// 镜像安全
		"trivy":      "trivy",
		"kube_bench": "kube_bench",
		// 构建工具
		"jenkins":  "jenkins",
		"tekton":   "tekton",
		"buildkit": "buildkit",
		// 部署目标
		"kubernetes": "kubernetes",
		"host":       "host",
		"host_group": "host_group",
		"kube-nova":  "kube-nova",
		"argocd":     "argocd",
	}

	if providerType, ok := mapping[channelTypeCode]; ok {
		return providerType
	}

	// 默认返回渠道类型本身
	return channelTypeCode
}

// convertOptionToPb 转换选项为pb格式
func convertOptionToPb(opt channelvars.Option) *pb.VariableOption {
	pbOpt := &pb.VariableOption{
		Label:    opt.Label,
		Value:    opt.Value,
		Disabled: opt.Disabled,
	}

	// 转换子选项
	if len(opt.Children) > 0 {
		pbOpt.Children = make([]*pb.VariableOption, len(opt.Children))
		for i, child := range opt.Children {
			pbOpt.Children[i] = convertOptionToPb(child)
		}
	}

	return pbOpt
}
