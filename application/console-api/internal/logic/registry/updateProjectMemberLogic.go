package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateProjectMemberLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新项目成员角色
func NewUpdateProjectMemberLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProjectMemberLogic {
	return &UpdateProjectMemberLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateProjectMemberLogic) UpdateProjectMember(req *types.UpdateProjectMemberRequest) (resp string, err error) {
	_, err = l.svcCtx.RepositoryRpc.UpdateProjectMember(l.ctx, &pb.UpdateProjectMemberReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
		MemberId:     req.MemberId,
		RoleId:       req.RoleId,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("更新项目成员角色成功: MemberId=%d, RoleId=%d", req.MemberId, req.RoleId)
	return "更新项目成员角色成功", nil
}
