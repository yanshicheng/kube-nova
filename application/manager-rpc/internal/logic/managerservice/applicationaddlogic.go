package managerservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
)

type ApplicationAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApplicationAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplicationAddLogic {
	return &ApplicationAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 项目的应用表相关rpc
// ApplicationAdd 添加项目应用
func (l *ApplicationAddLogic) ApplicationAdd(in *pb.AddOnecProjectApplicationReq) (*pb.AddOnecProjectApplicationResp, error) {
	// 参数校验
	if in.WorkspaceId == 0 {
		l.Errorf("参数校验失败: workspaceId 不能为空")
		return nil, errorx.Msg("workspaceId 不能为空")
	}
	if in.NameEn == "" {
		l.Errorf("参数校验失败: nameEn 不能为空")
		return nil, errorx.Msg("服务英文名不能为空")
	}
	if in.ResourceType == "" {
		l.Errorf("参数校验失败: resourceType 不能为空")
		return nil, errorx.Msg("服务资源类型不能为空")
	}
	// 检查 workspace 是否存在
	_, err := l.svcCtx.OnecProjectWorkspaceModel.FindOne(l.ctx, in.WorkspaceId)
	if err != nil {
		l.Errorf("查询工作空间失败: %v", err)
		return nil, status.Error(codes.Internal, "查询工作空间失败")
	}
	if !utils.IsResourceType(in.ResourceType) {
		l.Errorf("参数校验失败: resourceType 不支持")
		return nil, errorx.Msg("资源类型不支持")
	}
	// 检查应用是否已存在
	existApp, err := l.svcCtx.OnecProjectApplication.FindOneByWorkspaceIdNameEnResourceType(
		l.ctx, in.WorkspaceId, in.NameEn, in.ResourceType)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("查询应用失败: %v", err)
		return nil, errorx.Msg("查询应用失败")
	}
	if existApp != nil {
		l.Errorf("应用已存在: workspaceId=%d, nameEn=%s, resourceType=%s",
			in.WorkspaceId, in.NameEn, in.ResourceType)
		return nil, errorx.Msg("该工作空间下已存在同名同类型的应用")
	}

	// 构建应用数据
	application := &model.OnecProjectApplication{
		WorkspaceId:  in.WorkspaceId,
		NameCn:       in.NameCn,
		NameEn:       in.NameEn,
		ResourceType: strings.ToLower(in.ResourceType),
		Description:  in.Description,
		CreatedBy:    in.CreatedBy,
		UpdatedBy:    in.UpdatedBy,
	}

	// 插入数据库
	result, err := l.svcCtx.OnecProjectApplication.Insert(l.ctx, application)
	if err != nil {
		l.Errorf("添加应用失败: %v, data: %+v", err, application)
		return nil, errorx.Msg("添加应用失败")
	}

	// 获取插入的ID
	id, err := result.LastInsertId()
	if err != nil {
		l.Errorf("获取应用ID失败: %v", err)
		return nil, errorx.Msg("添加应用失败")
	} else {
		l.Infof("应用添加成功: id=%d, workspaceId=%d, nameEn=%s, nameCn=%s, resourceType=%s",
			id, in.WorkspaceId, in.NameEn, in.NameCn, in.ResourceType)
	}

	return &pb.AddOnecProjectApplicationResp{
		Id: uint64(id),
	}, nil
}
