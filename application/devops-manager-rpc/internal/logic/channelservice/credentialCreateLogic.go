package channelservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type CredentialCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCredentialCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CredentialCreateLogic {
	return &CredentialCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CredentialCreateLogic) CredentialCreate(in *pb.CreateCredentialReq) (*pb.IdResp, error) {
	if _, err := l.svcCtx.CredentialModel.FindOneByCode(l.ctx, in.Code); err == nil {
		l.Errorf("凭据编码已存在")
		return nil, errorx.Msg("凭据编码已存在")
	} else if !errors.Is(err, model.ErrNotFound) {
		l.Errorf("凭证创建失败: %v", err)
		return nil, err
	}
	scope := normalizeCredentialScope(in.Scope, in.ProjectId, false)
	if err := ensureCredentialTargetAccess(l.ctx, l.svcCtx, scope, in.ProjectId, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("凭证创建失败: %v", err)
		return nil, err
	}
	channelGroupCode := strings.TrimSpace(in.ChannelGroupCode)
	channelType := strings.TrimSpace(in.ChannelType)
	if err := validateCredentialChannelTarget(l.ctx, l.svcCtx, channelGroupCode, channelType, in.CredentialType); err != nil {
		l.Errorf("凭证创建失败: %v", err)
		return nil, err
	}
	data := &model.DevopsCredential{
		Name:             in.Name,
		Code:             in.Code,
		CredentialType:   in.CredentialType,
		Username:         in.Username,
		Password:         in.Password,
		Token:            in.Token,
		PrivateKey:       in.PrivateKey,
		Passphrase:       in.Passphrase,
		Kubeconfig:       in.Kubeconfig,
		SecretText:       in.SecretText,
		Certificate:      in.Certificate,
		JsonData:         in.JsonData,
		Description:      in.Description,
		Status:           in.Status,
		IsSystem:         scope == "system",
		Scope:            scope,
		ProjectID:        in.ProjectId,
		ChannelGroupCode: channelGroupCode,
		ChannelType:      channelType,
		CreatedBy:        in.CreatedBy,
		UpdatedBy:        in.CreatedBy,
	}
	if scope == "system" {
		data.ProjectID = ""
	}
	if data.Status == 0 {
		data.Status = 1
	}
	if err := validateCredentialPayload(data); err != nil {
		l.Errorf("凭证创建失败: %v", err)
		return nil, err
	}
	if err := l.svcCtx.CredentialModel.Insert(l.ctx, data); err != nil {
		if model.IsDuplicateKey(err) {
			l.Errorf("凭据编码已存在")
			return nil, errorx.Msg("凭据编码已存在")
		}
		l.Errorf("凭证创建失败: %v", err)
		return nil, err
	}

	return &pb.IdResp{Id: data.ID.Hex()}, nil
}
