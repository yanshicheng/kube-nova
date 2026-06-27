package logic

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"

	"github.com/zeromicro/go-zero/core/logx"
)

type RenderOutputLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRenderOutputLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RenderOutputLogic {
	return &RenderOutputLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RenderOutput 渲染输出
func (l *RenderOutputLogic) RenderOutput(in *pb.RenderOutputReq) (*pb.RenderOutputResp, error) {
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

	// 4. 调用Provider渲染输出
	req := &channelvars.RenderOutputRequest{
		ChannelInstanceID: in.ChannelInstanceId,
		ProviderKey:       in.ProviderKey,
		Values:            in.Values,
		ProjectID:         in.ProjectId,
	}

	resp, err := provider.RenderOutput(l.ctx, req)
	if err != nil {
		l.Errorf("渲染输出失败: %v", err)
		return nil, err
	}

	return &pb.RenderOutputResp{
		Output: resp.Output,
	}, nil
}
