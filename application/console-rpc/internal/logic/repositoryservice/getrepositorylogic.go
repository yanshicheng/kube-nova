package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRepositoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRepositoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRepositoryLogic {
	return &GetRepositoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetRepositoryLogic) GetRepository(in *pb.GetRepositoryReq) (*pb.GetRepositoryResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	repo, err := client.Repository().Get(in.ProjectName, in.RepoName)
	if err != nil {
		return nil, errorx.Msg("查询仓库失败")
	}

	return &pb.GetRepositoryResp{
		Data: &pb.Repository{
			Id:            repo.ID,
			Name:          repo.Name,
			ProjectId:     repo.ProjectID,
			ProjectName:   in.ProjectName,
			Description:   repo.Description,
			ArtifactCount: repo.ArtifactCount,
			PullCount:     repo.PullCount,
			CreationTime:  repo.CreationTime.Unix(),
			UpdateTime:    repo.UpdateTime.Unix(),
		},
	}, nil
}
