package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type HostTestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewHostTestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HostTestLogic {
	return &HostTestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *HostTestLogic) HostTest(in *pb.GetByIdReq) (*pb.TestChannelResp, error) {
	host, err := l.svcCtx.HostModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("主机测试失败: %v", err)
		return nil, err
	}
	result := l.svcCtx.ChannelChecker.TestHost(l.ctx, host)
	if err := l.svcCtx.HostModel.UpdateHealth(l.ctx, in.Id, result.HealthStatus, result.Message, result.Metadata, "system"); err != nil {
		l.Errorf("主机连通性状态更新失败: %v", err)
	}

	return &pb.TestChannelResp{
		Success:      result.Success,
		HealthStatus: result.HealthStatus,
		Message:      result.Message,
		CheckedAt:    result.CheckedAt,
		Metadata:     result.Metadata,
	}, nil
}
