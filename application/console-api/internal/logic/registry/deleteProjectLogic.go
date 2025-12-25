package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除仓库项目
func NewDeleteProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteProjectLogic {
	return &DeleteProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteProjectLogic) DeleteProject(req *types.DeleteProjectRequest) (resp string, err error) {
	_, err = l.svcCtx.RepositoryRpc.DeleteProject(l.ctx, &pb.DeleteProjectReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("删除仓库项目成功: ProjectName=%s", req.ProjectName)
	return "删除仓库项目成功", nil
}
