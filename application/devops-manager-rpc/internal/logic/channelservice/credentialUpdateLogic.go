package channelservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CredentialUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCredentialUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CredentialUpdateLogic {
	return &CredentialUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CredentialUpdateLogic) CredentialUpdate(in *pb.UpdateCredentialReq) (*pb.EmptyResp, error) {
	exist, err := l.svcCtx.CredentialModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("凭证更新失败: %v", err)
		return nil, err
	}
	if err := ensureCredentialWriteAccess(l.ctx, l.svcCtx, exist, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("凭证更新失败: %v", err)
		return nil, err
	}
	scope := normalizeCredentialScope(in.Scope, in.ProjectId, exist.IsSystem)
	projectID := in.ProjectId
	if scope == "system" {
		projectID = ""
	}
	if err := ensureCredentialTargetAccess(l.ctx, l.svcCtx, scope, projectID, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("凭证更新失败: %v", err)
		return nil, err
	}
	channelGroupCode := strings.TrimSpace(in.ChannelGroupCode)
	channelType := strings.TrimSpace(in.ChannelType)
	if err := validateCredentialChannelTarget(l.ctx, l.svcCtx, channelGroupCode, channelType, in.CredentialType); err != nil {
		l.Errorf("凭证更新失败: %v", err)
		return nil, err
	}
	data := &model.DevopsCredential{
		ID:               exist.ID,
		Name:             in.Name,
		CredentialType:   in.CredentialType,
		Username:         in.Username,
		Password:         keepMaskedSecret(in.Password, exist.Password),
		Token:            keepMaskedSecret(in.Token, exist.Token),
		PrivateKey:       keepMaskedSecret(in.PrivateKey, exist.PrivateKey),
		Passphrase:       keepMaskedSecret(in.Passphrase, exist.Passphrase),
		Kubeconfig:       keepMaskedSecret(in.Kubeconfig, exist.Kubeconfig),
		SecretText:       keepMaskedSecret(in.SecretText, exist.SecretText),
		Certificate:      keepMaskedSecret(in.Certificate, exist.Certificate),
		JsonData:         keepMaskedSecret(in.JsonData, exist.JsonData),
		Description:      in.Description,
		Status:           in.Status,
		IsSystem:         scope == "system",
		Scope:            scope,
		ProjectID:        projectID,
		ChannelGroupCode: channelGroupCode,
		ChannelType:      channelType,
		UpdatedBy:        in.UpdatedBy,
	}
	if err := validateCredentialPayload(data); err != nil {
		l.Errorf("凭证更新失败: %v", err)
		return nil, err
	}
	if err := l.svcCtx.CredentialModel.Update(l.ctx, data); err != nil {
		l.Errorf("凭证更新失败: %v", err)
		return nil, err
	}
	if err := l.svcCtx.CredentialSyncModel.MarkPendingByCredential(l.ctx, in.Id, "平台凭证已更新，等待重新同步", in.UpdatedBy); err != nil {
		l.Errorf("凭证更新失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
