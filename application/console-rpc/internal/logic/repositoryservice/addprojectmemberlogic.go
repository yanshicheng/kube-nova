package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddProjectMemberLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddProjectMemberLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddProjectMemberLogic {
	return &AddProjectMemberLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AddProjectMemberLogic) AddProjectMember(in *pb.AddProjectMemberReq) (*pb.AddProjectMemberResp, error) {
	// 获取 Harbor 客户端
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	// 构建请求
	req := &types.ProjectMemberReq{
		MemberUser: in.MemberUser,
		RoleID:     in.RoleId,
	}

	// 添加成员
	id, err := client.Member().Add(in.ProjectName, req)
	if err != nil {
		return nil, errorx.Msg("添加项目成员失败: " + err.Error())
	}

	return &pb.AddProjectMemberResp{
		Id:      id,
		Message: "添加成功",
	}, nil
}
