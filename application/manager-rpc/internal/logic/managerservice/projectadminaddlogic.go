package managerservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ProjectAdminAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectAdminAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectAdminAddLogic {
	return &ProjectAdminAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectAdminAdd 添加项目管理员（批量）
func (l *ProjectAdminAddLogic) ProjectAdminAdd(in *pb.AddOnecProjectAdminReq) (*pb.AddOnecProjectAdminResp, error) {
	// 参数校验
	if in.ProjectId == 0 {
		return nil, errorx.Msg("项目ID不能为空")
	}

	// 检查项目是否存在
	_, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, in.ProjectId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("项目不存在")
		}
		l.Error("查询项目失败", err)
		return nil, errorx.Msg("查询项目失败")
	}

	// ============ 重要：在事务前查询所有要删除的管理员记录 ============
	// 保存旧的管理员数据，用于后续缓存清理
	oldAdmins, err := l.queryProjectAdmins(l.ctx, in.ProjectId)
	if err != nil {
		l.Errorf("查询旧管理员记录失败: %v", err)
		return nil, errorx.Msg("查询管理员失败")
	}
	l.Infof("查询到 %d 条旧管理员记录，准备删除", len(oldAdmins))
	// ================================================================

	// 使用事务处理：先删除所有，再批量添加
	err = l.svcCtx.OnecProjectAdminModel.TransCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		// 删除该项目的所有管理员
		deleteQuery := "DELETE FROM `onec_project_admin` WHERE `project_id` = ? AND `is_deleted` = 0"
		_, err := session.ExecCtx(ctx, deleteQuery, in.ProjectId)
		if err != nil {
			l.Error("删除项目管理员失败", err)
			return err
		}

		// 批量添加新的管理员
		for _, userId := range in.UserIds {
			admin := &model.OnecProjectAdmin{
				ProjectId: in.ProjectId,
				UserId:    userId,
			}

			// 使用session执行插入
			insertQuery := "INSERT INTO `onec_project_admin` (`project_id`, `user_id`) VALUES (?, ?)"
			_, err = session.ExecCtx(ctx, insertQuery, admin.ProjectId, admin.UserId)
			if err != nil {
				l.Error("添加项目管理员失败", err, "userId", userId)
				return err
			}
		}

		return nil
	})

	if err != nil {
		l.Error("事务执行失败", err)
		return nil, errorx.Msg("添加项目管理员失败")
	}

	// ============ 事务成功后手动删除所有相关缓存 ============
	l.Infof("事务执行成功，开始清理缓存")

	// 1. 删除所有旧管理员记录的缓存
	l.Infof("开始删除 %d 条旧管理员记录的缓存", len(oldAdmins))
	for _, oldAdmin := range oldAdmins {
		if err := l.svcCtx.OnecProjectAdminModel.DeleteCache(l.ctx, oldAdmin.Id); err != nil {
			// 缓存删除失败只记录日志，不影响主流程
			l.Errorf("删除旧管理员缓存失败 [id=%d, projectId=%d, userId=%d]: %v",
				oldAdmin.Id, oldAdmin.ProjectId, oldAdmin.UserId, err)
		} else {
			l.Infof("成功删除旧管理员缓存 [id=%d, projectId=%d, userId=%d]",
				oldAdmin.Id, oldAdmin.ProjectId, oldAdmin.UserId)
		}
	}

	// 2. 删除所有新管理员的联合索引缓存
	// 注意：新记录的 ID 缓存无法删除（因为还不知道 ID），但联合索引缓存可以删除
	l.Infof("开始删除 %d 条新管理员记录的联合索引缓存", len(in.UserIds))
	for _, userId := range in.UserIds {
		if err := l.svcCtx.OnecProjectAdminModel.DeleteCacheByProjectIdUserId(
			l.ctx, in.ProjectId, userId); err != nil {
			l.Errorf("删除新管理员联合索引缓存失败 [projectId=%d, userId=%d]: %v",
				in.ProjectId, userId, err)
		} else {
			l.Infof("成功删除新管理员联合索引缓存 [projectId=%d, userId=%d]",
				in.ProjectId, userId)
		}
	}

	l.Infof("缓存清理完成")
	// ========================================================

	return &pb.AddOnecProjectAdminResp{}, nil
}

// queryProjectAdmins 查询项目的所有管理员记录（事务前调用）
func (l *ProjectAdminAddLogic) queryProjectAdmins(ctx context.Context, projectId uint64) ([]*model.OnecProjectAdmin, error) {
	// 使用 SearchNoPage 查询该项目的所有管理员
	queryStr := "`project_id` = ?"
	admins, err := l.svcCtx.OnecProjectAdminModel.SearchNoPage(ctx, "", true, queryStr, projectId)
	if err != nil {
		// 如果没有找到记录，返回空数组（不是错误）
		if errors.Is(err, model.ErrNotFound) {
			return []*model.OnecProjectAdmin{}, nil
		}
		return nil, err
	}

	return admins, nil
}
