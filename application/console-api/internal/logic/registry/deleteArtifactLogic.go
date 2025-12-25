package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteArtifactLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除制品
func NewDeleteArtifactLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteArtifactLogic {
	return &DeleteArtifactLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteArtifactLogic) DeleteArtifact(req *types.DeleteArtifactRequest) (resp string, err error) {
	_, err = l.svcCtx.RepositoryRpc.DeleteArtifact(l.ctx, &pb.DeleteArtifactReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
		RepoName:     req.RepoName,
		Reference:    req.Reference,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("删除制品成功: Reference=%s", req.Reference)
	return "删除制品成功", nil
}
