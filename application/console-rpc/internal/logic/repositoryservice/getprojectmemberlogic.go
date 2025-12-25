package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProjectMemberLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProjectMemberLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectMemberLogic {
	return &GetProjectMemberLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetProjectMemberLogic) GetProjectMember(in *pb.GetProjectMemberReq) (*pb.GetProjectMemberResp, error) {
	// 获取 Harbor 客户端
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	// 获取成员详情
	member, err := client.Member().Get(in.ProjectName, in.MemberId)
	if err != nil {
		return nil, errorx.Msg("查询项目成员失败")
	}

	return &pb.GetProjectMemberResp{
		Data: &pb.ProjectMember{
			Id:           member.ID,
			ProjectId:    member.ProjectID,
			EntityName:   member.EntityName,
			EntityType:   member.EntityType,
			RoleId:       member.RoleID,
			RoleName:     member.RoleName,
			CreationTime: member.CreationTime.Unix(),
			UpdateTime:   member.UpdateTime.Unix(),
		},
	}, nil
}
