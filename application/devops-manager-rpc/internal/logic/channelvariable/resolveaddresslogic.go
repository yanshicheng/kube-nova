package logic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResolveAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveAddressLogic {
	return &ResolveAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ResolveAddress 解析地址
func (l *ResolveAddressLogic) ResolveAddress(in *pb.ResolveAddressReq) (*pb.ResolveAddressResp, error) {
	// 创建地址解析器
	resolver := channelvars.NewAddressResolver(l.svcCtx.ChannelManagerAdapter)

	// 解析地址
	req := &channelvars.ResolveAddressRequest{
		Address:         in.Address,
		ChannelTypeCode: in.ChannelTypeCode,
		ProjectID:       in.ProjectId,
	}

	resp, err := resolver.Resolve(l.ctx, req)
	if err != nil {
		l.Errorf("解析地址失败: %v", err)
		return nil, err
	}

	return &pb.ResolveAddressResp{
		Resolved:          resp.Resolved,
		ChannelInstanceId: resp.ChannelInstanceID,
		Fields:            resp.Fields,
		Reason:            resp.Reason,
	}, nil
}
