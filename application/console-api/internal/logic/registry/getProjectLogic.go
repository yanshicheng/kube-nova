package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取项目详情
func NewGetProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectLogic {
	return &GetProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProjectLogic) GetProject(req *types.GetProjectRequest) (resp *types.GetProjectResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.GetProject(l.ctx, &pb.GetProjectReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	data := rpcResp.Data
	l.Infof("获取项目详情成功: ProjectName=%s, RepoCount=%d", data.Name, data.RepoCount)

	return &types.GetProjectResponse{
		Data: types.RegistryProject{
			ProjectId:    data.ProjectId,
			Name:         data.Name,
			OwnerName:    data.OwnerName,
			IsPublic:     data.IsPublic,
			RepoCount:    data.RepoCount,
			CreationTime: data.CreationTime,
			UpdateTime:   data.UpdateTime,
		},
	}, nil
}
