package portalservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type APIAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAPIAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *APIAddLogic {
	return &APIAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// -----------------------系统API权限表-----------------------
func (l *APIAddLogic) APIAdd(in *pb.AddSysAPIReq) (*pb.AddSysAPIResp, error) {
	// 参数验证
	if in.Name == "" {
		l.Error("API名称不能为空")
		return nil, errorx.Msg("API名称不能为空")
	}
	if in.IsPermission == 1 {

		if in.Path == "" {
			l.Error("API路径不能为空")
			return nil, errorx.Msg("API路径不能为空")
		}
		if in.Method == "" {
			l.Error("HTTP方法不能为空")
			return nil, errorx.Msg("HTTP方法不能为空")
		}
		// 检查API是否已存在（路径+方法的组合应该是唯一的）
		existApi, _ := l.svcCtx.SysApi.FindOneByPathMethod(l.ctx, in.Path, in.Method)
		if existApi != nil {
			l.Errorf("API已存在，路径: %s, 方法: %s", in.Path, in.Method)
			return nil, errorx.Msg("该API已存在")
		}
	}

	// 如果有父级ID，验证父级是否存在
	if in.ParentId > 0 {
		_, err := l.svcCtx.SysApi.FindOne(l.ctx, in.ParentId)
		if err != nil {
			l.Errorf("父级API不存在，父级ID: %d, 错误: %v", in.ParentId, err)
			return nil, errorx.Msg("父级API不存在")
		}
	}

	// 构建API数据
	sysApi := &model.SysApi{
		ParentId:     in.ParentId,
		Name:         in.Name,
		Path:         in.Path,
		Method:       in.Method,
		IsPermission: in.IsPermission,
		CreatedBy:    in.CreatedBy,
		UpdatedBy:    in.UpdatedBy,
	}

	// 插入数据库
	result, err := l.svcCtx.SysApi.Insert(l.ctx, sysApi)
	if err != nil {
		l.Errorf("插入API失败，错误: %v", err)
		return nil, errorx.Msg("添加API失败")
	}

	apiId, _ := result.LastInsertId()
	l.Infof("API添加成功，APIID: %d, API名称: %s", apiId, in.Name)

	return &pb.AddSysAPIResp{}, nil
}
