package logic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/devops/channelvars"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResolveCredentialLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveCredentialLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveCredentialLogic {
	return &ResolveCredentialLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ResolveCredential 解析凭证
func (l *ResolveCredentialLogic) ResolveCredential(in *pb.ResolveCredentialReq) (*pb.ResolveCredentialResp, error) {
	// 创建凭证解析器
	resolver := channelvars.NewCredentialResolver(
		l.svcCtx.ChannelManagerAdapter,
		l.svcCtx.CredentialManagerAdapter,
	)

	// 解析凭证
	req := &channelvars.ResolveCredentialRequest{
		EndpointID:      in.EndpointId,
		Address:         in.Address,
		ChannelTypeCode: in.ChannelTypeCode,
		ProjectID:       in.ProjectId,
	}

	credential, err := resolver.Resolve(l.ctx, req)
	if err != nil {
		l.Errorf("解析凭证失败: %v", err)
		return nil, err
	}

	return &pb.ResolveCredentialResp{
		CredentialId:   credential.ID,
		CredentialName: credential.Name,
		CredentialType: credential.Type,
	}, nil
}
