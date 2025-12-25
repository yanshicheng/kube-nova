package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteProjectMemberLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteProjectMemberLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteProjectMemberLogic {
	return &DeleteProjectMemberLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteProjectMemberLogic) DeleteProjectMember(in *pb.DeleteProjectMemberReq) (*pb.DeleteProjectMemberResp, error) {
	// 获取 Harbor 客户端
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	// 删除成员
	err = client.Member().Delete(in.ProjectName, in.MemberId)
	if err != nil {
		return nil, errorx.Msg("删除项目成员失败: " + err.Error())
	}

	return &pb.DeleteProjectMemberResp{
		Message: "删除成功",
	}, nil
}
