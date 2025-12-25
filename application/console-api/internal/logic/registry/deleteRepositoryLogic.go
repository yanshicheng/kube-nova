package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteRepositoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除仓库
func NewDeleteRepositoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteRepositoryLogic {
	return &DeleteRepositoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteRepositoryLogic) DeleteRepository(req *types.DeleteRepositoryRequest) (resp string, err error) {
	_, err = l.svcCtx.RepositoryRpc.DeleteRepository(l.ctx, &pb.DeleteRepositoryReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
		RepoName:     req.RepoName,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("删除仓库成功: RepoName=%s", req.RepoName)
	return "删除仓库成功", nil
}
