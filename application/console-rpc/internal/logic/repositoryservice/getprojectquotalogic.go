package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProjectQuotaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProjectQuotaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectQuotaLogic {
	return &GetProjectQuotaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ============ 配额管理 ============
func (l *GetProjectQuotaLogic) GetProjectQuota(in *pb.GetProjectQuotaReq) (*pb.GetProjectQuotaResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	quota, err := client.Quota().GetByProject(in.ProjectName)
	if err != nil {
		return nil, errorx.Msg("查询项目配额失败")
	}

	return &pb.GetProjectQuotaResp{
		Data: &pb.ProjectQuota{
			StorageLimit: quota.Hard.Storage,
			StorageUsed:  quota.Used.Storage,
			CountLimit:   quota.Hard.Count,
			CountUsed:    quota.Used.Count,
		},
	}, nil
}
