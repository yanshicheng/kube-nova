package menu

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSysMenuByIdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSysMenuByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSysMenuByIdLogic {
	return &GetSysMenuByIdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// menu/getSysMenuByIdLogic.go
func (l *GetSysMenuByIdLogic) GetSysMenuById(req *types.DefaultIdRequest) (resp *types.SysMenu, err error) {

	// 调用 RPC 服务获取菜单信息
	res, err := l.svcCtx.PortalRpc.MenuGetById(l.ctx, &pb.GetSysMenuByIdReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取菜单失败: menuId=%d, error=%v", req.Id, err)
		return nil, err
	}

	l.Infof("获取菜单成功: menuId=%d, menuName=%s", req.Id, res.Data.Name)

	return &types.SysMenu{
		Id:            res.Data.Id,
		ParentId:      res.Data.ParentId,
		MenuType:      res.Data.MenuType,
		Name:          res.Data.Name,
		Path:          res.Data.Path,
		Component:     res.Data.Component,
		Redirect:      res.Data.Redirect,
		Label:         res.Data.Label,
		Title:         res.Data.Title,
		Icon:          res.Data.Icon,
		Sort:          res.Data.Sort,
		Link:          res.Data.Link,
		IsEnable:      res.Data.IsEnable,
		IsMenu:        res.Data.IsMenu,
		KeepAlive:     res.Data.KeepAlive,
		IsHide:        res.Data.IsHide,
		IsIframe:      res.Data.IsIframe,
		IsHideTab:     res.Data.IsHideTab,
		ShowBadge:     res.Data.ShowBadge,
		ShowTextBadge: res.Data.ShowTextBadge,
		IsFirstLevel:  res.Data.IsFirstLevel,
		FixedTab:      res.Data.FixedTab,
		IsFullPage:    res.Data.IsFullPage,
		ActivePath:    res.Data.ActivePath,
		Roles:         res.Data.Roles,
		AuthName:      res.Data.AuthName,
		AuthLabel:     res.Data.AuthLabel,
		AuthIcon:      res.Data.AuthIcon,
		AuthSort:      res.Data.AuthSort,
		Status:        res.Data.Status,
		CreatedAt:     res.Data.CreateTime,
		UpdatedAt:     res.Data.UpdateTime,
		CreatedBy:     res.Data.CreateBy,
		UpdatedBy:     res.Data.UpdateBy,
	}, nil
}
