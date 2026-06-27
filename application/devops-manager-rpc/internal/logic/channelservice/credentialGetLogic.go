package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CredentialGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCredentialGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CredentialGetLogic {
	return &CredentialGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CredentialGetLogic) CredentialGet(in *pb.GetByIdReq) (*pb.GetCredentialResp, error) {
	data, err := l.svcCtx.CredentialModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("凭证查询详情失败: %v", err)
		return nil, err
	}
	if err := ensureCredentialReadAccess(l.ctx, l.svcCtx, data, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("凭证查询详情失败: %v", err)
		return nil, err
	}
	out := credentialToPb(data)
	syncMap, err := l.svcCtx.CredentialSyncModel.LatestByCredentialIDs(l.ctx, []string{data.ID.Hex()})
	if err != nil {
		l.Errorf("凭证查询详情失败: %v", err)
		return nil, err
	}
	applyCredentialSync(out, syncMap[data.ID.Hex()])

	return &pb.GetCredentialResp{Data: out}, nil
}
