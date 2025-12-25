package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type AddProjectMemberLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 添加项目成员
func NewAddProjectMemberLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddProjectMemberLogic {
	return &AddProjectMemberLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddProjectMemberLogic) AddProjectMember(req *types.AddProjectMemberRequest) (resp string, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.AddProjectMember(l.ctx, &pb.AddProjectMemberReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
		MemberUser:   req.MemberUser,
		RoleId:       req.RoleId,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("添加项目成员成功: MemberId=%d, MemberUser=%s", rpcResp.Id, req.MemberUser)
	return "添加项目成员成功", nil
}
