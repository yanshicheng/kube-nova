package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteTagLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除标签
func NewDeleteTagLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteTagLogic {
	return &DeleteTagLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteTagLogic) DeleteTag(req *types.DeleteTagRequest) (resp string, err error) {

	_, err = l.svcCtx.RepositoryRpc.DeleteTag(l.ctx, &pb.DeleteTagReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
		RepoName:     req.RepoName,
		TagName:      req.TagName,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("删除标签成功: TagName=%s", req.TagName)
	return "删除标签成功", nil
}
