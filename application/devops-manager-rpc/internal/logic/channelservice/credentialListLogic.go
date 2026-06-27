package channelservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CredentialListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCredentialListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CredentialListLogic {
	return &CredentialListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CredentialListLogic) CredentialList(in *pb.ListCredentialReq) (*pb.ListCredentialResp, error) {
	projectIDs, restricted, err := userProjectIDs(l.ctx, l.svcCtx, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("凭证查询列表失败: %v", err)
		return nil, err
	}
	list, total, err := l.svcCtx.CredentialModel.List(l.ctx, model.DevopsCredentialListFilter{
		Name:             in.Name,
		Code:             in.Code,
		CredentialType:   in.CredentialType,
		Status:           in.Status,
		Scope:            in.Scope,
		ProjectID:        in.ProjectId,
		ProjectIDs:       projectIDs,
		ChannelGroupCode: strings.TrimSpace(in.ChannelGroupCode),
		ChannelType:      strings.TrimSpace(in.ChannelType),
		Restricted:       restricted,
		Page:             in.Page,
		PageSize:         in.PageSize,
	})
	if err != nil {
		l.Errorf("凭证查询列表失败: %v", err)
		return nil, err
	}
	credentialIDs := make([]string, 0, len(list))
	for _, item := range list {
		if item != nil {
			credentialIDs = append(credentialIDs, item.ID.Hex())
		}
	}
	syncMap, err := l.svcCtx.CredentialSyncModel.LatestByCredentialIDsAndBinding(l.ctx, credentialIDs, in.BuildChannelBindingId)
	if err != nil {
		l.Errorf("凭证查询列表失败: %v", err)
		return nil, err
	}
	items := make([]*pb.DevopsCredential, 0, len(list))
	for _, item := range list {
		out := credentialToPb(item)
		applyCredentialSync(out, syncMap[item.ID.Hex()])
		items = append(items, out)
	}

	return &pb.ListCredentialResp{Data: items, Total: total}, nil
}
