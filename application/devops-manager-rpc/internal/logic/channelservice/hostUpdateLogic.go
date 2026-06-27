package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type HostUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewHostUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HostUpdateLogic {
	return &HostUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *HostUpdateLogic) HostUpdate(in *pb.UpdateHostReq) (*pb.EmptyResp, error) {
	exist, err := l.svcCtx.HostModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("主机更新失败: %v", err)
		return nil, err
	}
	if err := validateHostCredential(l.ctx, l.svcCtx, in.CredentialId); err != nil {
		l.Errorf("主机更新失败: %v", err)
		return nil, err
	}
	data := &model.DevopsHost{
		ID:           exist.ID,
		Name:         in.Name,
		IP:           in.Ip,
		Port:         in.Port,
		CredentialID: in.CredentialId,
		Labels:       in.Labels,
		Description:  in.Description,
		Status:       in.Status,
		UpdatedBy:    in.UpdatedBy,
	}
	if err := l.svcCtx.HostModel.Update(l.ctx, data); err != nil {
		l.Errorf("主机更新失败: %v", err)
		return nil, err
	}
	result := l.svcCtx.ChannelChecker.TestHost(l.ctx, data)
	if err := l.svcCtx.HostModel.UpdateHealth(l.ctx, data.ID.Hex(), result.HealthStatus, result.Message, result.Metadata, in.UpdatedBy); err != nil {
		l.Errorf("主机连通性状态更新失败: %v", err)
	}

	return &pb.EmptyResp{}, nil
}
