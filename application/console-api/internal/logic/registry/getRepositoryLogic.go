package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetRepositoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取仓库详情
func NewGetRepositoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRepositoryLogic {
	return &GetRepositoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetRepositoryLogic) GetRepository(req *types.GetRepositoryRequest) (resp *types.GetRepositoryResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.GetRepository(l.ctx, &pb.GetRepositoryReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
		RepoName:     req.RepoName,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	data := rpcResp.Data
	l.Infof("获取仓库详情成功: RepoName=%s, ArtifactCount=%d", data.Name, data.ArtifactCount)

	return &types.GetRepositoryResponse{
		Data: types.Repository{
			Id:            data.Id,
			Name:          data.Name,
			ProjectId:     data.ProjectId,
			ProjectName:   data.ProjectName,
			Description:   data.Description,
			ArtifactCount: data.ArtifactCount,
			PullCount:     data.PullCount,
			CreationTime:  data.CreationTime,
			UpdateTime:    data.UpdateTime,
		},
	}, nil
}
