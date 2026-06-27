package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type HostTestConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewHostTestConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HostTestConfigLogic {
	return &HostTestConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *HostTestConfigLogic) HostTestConfig(in *pb.TestHostReq) (*pb.TestChannelResp, error) {
	if err := validateHostCredential(l.ctx, l.svcCtx, in.CredentialId); err != nil {
		l.Errorf("主机测试配置失败: %v", err)
		return nil, err
	}
	host := &model.DevopsHost{
		Name:         in.Name,
		IP:           in.Ip,
		Port:         in.Port,
		CredentialID: in.CredentialId,
	}
	result := l.svcCtx.ChannelChecker.TestHost(l.ctx, host)
	return &pb.TestChannelResp{
		Success:      result.Success,
		HealthStatus: result.HealthStatus,
		Message:      result.Message,
		CheckedAt:    result.CheckedAt,
		Metadata:     result.Metadata,
	}, nil
}
