package portalservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type APIDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAPIDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *APIDelLogic {
	return &APIDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// APIDel 删除 API 权限
func (l *APIDelLogic) APIDel(in *pb.DelSysAPIReq) (*pb.DelSysAPIResp, error) {
	// 参数验证
	if in.Id == 0 {
		l.Error("API ID 不能为空")
		return nil, errorx.Msg("API ID 不能为空")
	}

	// 验证 API 是否存在
	existApi, err := l.svcCtx.SysApi.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("API 不存在，API ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("API 不存在")
	}

	// 检查是否有子 API
	childApis, err := l.svcCtx.SysApi.SearchNoPage(l.ctx, "id", true, "parent_id = ?", in.Id)
	if err == nil && len(childApis) > 0 {
		l.Errorf("该 API 下还有子 API，无法删除，API ID: %d, 子 API 数量: %d", in.Id, len(childApis))
		return nil, errorx.Msg("该 API 下还有子 API，请先删除子 API")
	}

	// 检查是否有角色在使用该 API
	roleApis, err := l.svcCtx.SysRoleApi.SearchNoPage(l.ctx, "", true, "api_id = ?", in.Id)
	if err == nil && len(roleApis) > 0 {
		l.Errorf("该 API 正在被角色使用，无法删除，API ID: %d, 使用角色数量: %d", in.Id, len(roleApis))
		return nil, errorx.Msg("该 API 正在被角色使用，无法删除")
	}

	// 执行软删除
	err = l.svcCtx.SysApi.DeleteSoft(l.ctx, in.Id)
	if err != nil {
		l.Errorf("删除 API 失败，API ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("删除 API 失败")
	}

	l.Infof("API 删除成功，API ID: %d, API 路径: %s", in.Id, existApi.Path)
	return &pb.DelSysAPIResp{}, nil
}
