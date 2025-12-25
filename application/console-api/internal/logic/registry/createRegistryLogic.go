package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateRegistryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建镜像仓库
func NewCreateRegistryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateRegistryLogic {
	return &CreateRegistryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateRegistryLogic) CreateRegistry(req *types.CreateRegistryRequest) (resp string, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.CreateRegistry(l.ctx, &pb.CreateRegistryReq{
		Name:        req.Name,
		Type:        req.Type,
		Env:         req.Env,
		Url:         req.Url,
		Username:    req.Username,
		Password:    req.Password,
		Insecure:    req.Insecure,
		CaCert:      req.CaCert,
		Config:      req.Config,
		Status:      req.Status,
		Description: req.Description,
		CreatedBy:   req.CreatedBy,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("镜像仓库创建成功: UUID=%s", rpcResp.Uuid)
	return "镜像仓库创建成功", nil
}
