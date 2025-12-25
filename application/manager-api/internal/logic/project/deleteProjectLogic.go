package project

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewDeleteProjectLogic 软删除项目
func NewDeleteProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteProjectLogic {
	return &DeleteProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteProjectLogic) DeleteProject(req *types.DefaultIdRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 先查询项目信息用于审计日志
	projectInfo, queryErr := l.svcCtx.ManagerRpc.ProjectGetById(l.ctx, &pb.GetOnecProjectByIdReq{
		Id: req.Id,
	})

	var projectName string
	if queryErr == nil && projectInfo.Data != nil {
		projectName = projectInfo.Data.Name
	}

	// 调用RPC服务删除项目
	_, err = l.svcCtx.ManagerRpc.ProjectDel(l.ctx, &pb.DelOnecProjectReq{
		Id: req.Id,
	})

	// 记录审计日志
	auditStatus := int64(1)
	if err != nil {
		auditStatus = 0
	}
	actionDetail := fmt.Sprintf("用户 %s 删除项目, 项目ID: %d, 名称: %s", username, req.Id, projectName)
	_, auditErr := l.svcCtx.ManagerRpc.ProjectAuditLogAdd(l.ctx, &managerservice.AddOnecProjectAuditLogReq{
		ProjectId:    uint64(req.Id),
		Title:        "删除项目",
		ActionDetail: actionDetail,
		Status:       auditStatus,
	})
	if auditErr != nil {
		l.Errorf("记录审计日志失败: %v", auditErr)
	}

	if err != nil {
		l.Errorf("删除项目失败: %v", err)
		return "", fmt.Errorf("删除项目失败: %v", err)
	}

	return "项目删除成功", nil
}
