package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteProjectMemberLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除项目成员
func NewDeleteProjectMemberLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteProjectMemberLogic {
	return &DeleteProjectMemberLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteProjectMemberLogic) DeleteProjectMember(req *types.DeleteProjectMemberRequest) (resp string, err error) {
	_, err = l.svcCtx.RepositoryRpc.DeleteProjectMember(l.ctx, &pb.DeleteProjectMemberReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
		MemberId:     req.MemberId,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("删除项目成员成功: MemberId=%d", req.MemberId)
	return "删除项目成员成功", nil
}
