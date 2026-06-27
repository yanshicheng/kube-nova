package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChannelTestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChannelTestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChannelTestLogic {
	return &ChannelTestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChannelTestLogic) ChannelTest(in *pb.TestChannelReq) (*pb.TestChannelResp, error) {
	channel := &model.DevopsChannel{
		GroupID:         in.GroupId,
		Name:            in.Name,
		Code:            in.Code,
		ChannelType:     in.ChannelType,
		Endpoint:        in.Endpoint,
		AuthType:        normalizeAuthType(in.AuthType),
		Username:        in.Username,
		Password:        in.Password,
		Token:           in.Token,
		InsecureSkipTLS: in.InsecureSkipTls,
		Config:          in.Config,
		CredentialID:    in.CredentialId,
	}
	if in.Id != "" {
		exist, err := l.svcCtx.ChannelModel.FindOne(l.ctx, in.Id)
		if err != nil {
			l.Errorf("渠道测试失败: %v", err)
			return nil, err
		}
		if channel.Endpoint == "" {
			channel.Endpoint = exist.Endpoint
		}
		if channel.ChannelType == "" {
			channel.ChannelType = exist.ChannelType
		}
		if channel.AuthType == "none" && exist.AuthType != "" {
			channel.AuthType = exist.AuthType
		}
		if channel.Username == "" {
			channel.Username = exist.Username
		}
		if channel.Password == "" || channel.Password == maskedSecret {
			channel.Password = exist.Password
		}
		if channel.Token == "" || channel.Token == maskedSecret {
			channel.Token = exist.Token
		}
		if channel.Config == "" {
			channel.Config = exist.Config
		}
		if channel.CredentialID == "" {
			channel.CredentialID = exist.CredentialID
		}
		if !channel.InsecureSkipTLS {
			channel.InsecureSkipTLS = exist.InsecureSkipTLS
		}
	}

	if channel.GroupID != "" && channel.ChannelType != "" {
		if err := validateChannelScope(l.ctx, l.svcCtx, channel.GroupID, channel.ChannelType, channel.CredentialID); err != nil {
			l.Errorf("渠道测试失败: %v", err)
			return nil, err
		}
	}
	result := l.svcCtx.ChannelChecker.Test(l.ctx, channel)
	if in.Id != "" {
		if err := l.svcCtx.ChannelModel.UpdateHealth(l.ctx, in.Id, result.HealthStatus, result.Message, result.Metadata, "system"); err != nil {
			l.Errorf("渠道连通性状态更新失败: %v", err)
		}
	}

	return &pb.TestChannelResp{
		Success:      result.Success,
		HealthStatus: result.HealthStatus,
		Message:      result.Message,
		CheckedAt:    result.CheckedAt,
		Metadata:     result.Metadata,
	}, nil
}
