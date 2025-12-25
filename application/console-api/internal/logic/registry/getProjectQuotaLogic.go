package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/client/repositoryservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProjectQuotaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取项目配额
func NewGetProjectQuotaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectQuotaLogic {
	return &GetProjectQuotaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProjectQuotaLogic) GetProjectQuota(req *types.GetProjectQuotaRequest) (resp *types.GetProjectQuotaResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.GetProjectQuota(l.ctx, &repositoryservice.GetProjectQuotaReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	data := rpcResp.Data
	l.Infof("获取项目配额成功: StorageLimit=%d, StorageUsed=%d", data.StorageLimit, data.StorageUsed)

	return &types.GetProjectQuotaResponse{
		Data: types.ProjectQuota{
			StorageLimit: data.StorageLimit,
			StorageUsed:  data.StorageUsed,
			CountLimit:   data.CountLimit,
			CountUsed:    data.CountUsed,
		},
	}, nil
}
