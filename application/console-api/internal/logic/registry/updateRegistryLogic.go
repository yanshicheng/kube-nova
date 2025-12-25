package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateRegistryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新镜像仓库
func NewUpdateRegistryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateRegistryLogic {
	return &UpdateRegistryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateRegistryLogic) UpdateRegistry(req *types.UpdateRegistryRequest) (resp string, err error) {
	_, err = l.svcCtx.RepositoryRpc.UpdateRegistry(l.ctx, &pb.UpdateRegistryReq{
		Id:          req.Id,
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
		UpdatedBy:   req.UpdatedBy,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("镜像仓库更新成功: ID=%d", req.Id)
	return "镜像仓库更新成功", nil
}
