package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateProjectMemberLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateProjectMemberLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProjectMemberLogic {
	return &UpdateProjectMemberLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateProjectMemberLogic) UpdateProjectMember(in *pb.UpdateProjectMemberReq) (*pb.UpdateProjectMemberResp, error) {
	// 获取 Harbor 客户端
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	// 构建请求
	req := &types.ProjectMemberReq{
		RoleID: in.RoleId,
	}

	// 更新成员角色
	err = client.Member().Update(in.ProjectName, in.MemberId, req)
	if err != nil {
		return nil, errorx.Msg("更新项目成员失败: " + err.Error())
	}

	return &pb.UpdateProjectMemberResp{
		Message: "更新成功",
	}, nil
}
