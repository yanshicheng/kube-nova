package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type HostCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewHostCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HostCreateLogic {
	return &HostCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *HostCreateLogic) HostCreate(in *pb.CreateHostReq) (*pb.IdResp, error) {
	if err := validateHostCredential(l.ctx, l.svcCtx, in.CredentialId); err != nil {
		l.Errorf("主机创建失败: %v", err)
		return nil, err
	}
	data := &model.DevopsHost{
		Name:         in.Name,
		IP:           in.Ip,
		Port:         in.Port,
		CredentialID: in.CredentialId,
		Labels:       in.Labels,
		Description:  in.Description,
		Status:       in.Status,
		CreatedBy:    in.CreatedBy,
		UpdatedBy:    in.CreatedBy,
	}
	if data.Status == 0 {
		data.Status = 1
	}
	if err := l.svcCtx.HostModel.Insert(l.ctx, data); err != nil {
		l.Errorf("主机创建失败: %v", err)
		return nil, err
	}
	result := l.svcCtx.ChannelChecker.TestHost(l.ctx, data)
	if err := l.svcCtx.HostModel.UpdateHealth(l.ctx, data.ID.Hex(), result.HealthStatus, result.Message, result.Metadata, in.CreatedBy); err != nil {
		l.Errorf("主机连通性状态更新失败: %v", err)
	}

	return &pb.IdResp{Id: data.ID.Hex()}, nil
}
