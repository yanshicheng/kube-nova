package api

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddSysAPILogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddSysAPILogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddSysAPILogic {
	return &AddSysAPILogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddSysAPILogic) AddSysAPI(req *types.AddSysAPIRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务添加API
	_, err = l.svcCtx.PortalRpc.APIAdd(l.ctx, &pb.AddSysAPIReq{
		ParentId:     req.ParentId,
		Name:         req.Name,
		Path:         req.Path,
		Method:       req.Method,
		IsPermission: req.IsPermission,
		CreatedBy:    username,
		UpdatedBy:    username,
	})
	if err != nil {
		l.Errorf("添加API失败: operator=%s, apiName=%s, error=%v", username, req.Name, err)
		return "", err
	}

	l.Infof("添加API成功: operator=%s, apiName=%s", username, req.Name)
	return "添加API成功", nil
}
