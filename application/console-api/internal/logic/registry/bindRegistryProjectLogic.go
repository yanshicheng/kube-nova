package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type BindRegistryProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 绑定仓库项目到应用项目
func NewBindRegistryProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindRegistryProjectLogic {
	return &BindRegistryProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BindRegistryProjectLogic) BindRegistryProject(req *types.BindRegistryProjectRequest) (resp string, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.BindRegistryProject(l.ctx, &pb.BindRegistryProjectReq{
		RegistryId:          req.RegistryId,
		AppProjectId:        req.AppProjectId,
		RegistryProjectName: req.RegistryProjectName,
		RegistryProjectId:   req.RegistryProjectId,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("绑定仓库项目成功: BindingId=%d", rpcResp.Id)
	return "绑定仓库项目成功", nil
}
