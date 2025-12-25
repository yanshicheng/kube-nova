package api

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSysAPILogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateSysAPILogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSysAPILogic {
	return &UpdateSysAPILogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateSysAPILogic) UpdateSysAPI(req *types.UpdateSysAPIRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务更新API
	_, err = l.svcCtx.PortalRpc.APIUpdate(l.ctx, &pb.UpdateSysAPIReq{
		Id:           req.Id,
		ParentId:     req.ParentId,
		Name:         req.Name,
		Path:         req.Path,
		Method:       req.Method,
		IsPermission: req.IsPermission,
		UpdatedBy:    username,
	})
	if err != nil {
		l.Errorf("更新API失败: operator=%s, apiId=%d, error=%v", username, req.Id, err)
		return "", err
	}

	return "更新API成功", nil
}
