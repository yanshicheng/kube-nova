package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建仓库项目
func NewCreateProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateProjectLogic {
	return &CreateProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateProjectLogic) CreateProject(req *types.CreateProjectRequest) (resp string, err error) {
	// 🔧 修复：设置默认单位
	storageUnit := req.StorageUnit
	if storageUnit == "" {
		storageUnit = "GB" // 默认为 GB
	}

	rpcResp, err := l.svcCtx.RepositoryRpc.CreateProject(l.ctx, &pb.CreateProjectReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
		IsPublic:     req.IsPublic,
		StorageLimit: req.StorageLimit,
		StorageUnit:  storageUnit,
		AppProjectId: req.AppProjectId,
		ClusterUuid:  req.ClusterUuid,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("创建仓库项目成功: ProjectId=%d, ProjectName=%s, StorageLimit=%d%s",
		rpcResp.ProjectId, req.ProjectName, req.StorageLimit, storageUnit)
	return "创建仓库项目成功", nil
}
