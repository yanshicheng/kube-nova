package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectLogic {
	return &GetProjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetProjectLogic) GetProject(in *pb.GetProjectReq) (*pb.GetProjectResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	project, err := client.Project().Get(in.ProjectName)
	if err != nil {
		return nil, errorx.Msg("查询项目失败")
	}

	return &pb.GetProjectResp{
		Data: &pb.RegistryProject{
			ProjectId:           project.ProjectID,
			Name:                project.Name,
			OwnerName:           project.OwnerName,
			IsPublic:            project.Public,
			RepoCount:           project.RepoCount,
			CreationTime:        project.CreationTime.Unix(),
			UpdateTime:          project.UpdateTime.Unix(),
			StorageLimit:        project.StorageLimit,
			StorageUsed:         project.StorageUsed,
			StorageLimitDisplay: project.StorageLimitDisplay,
			StorageUsedDisplay:  project.StorageUsedDisplay,
		},
	}, nil
}
