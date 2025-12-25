package portalservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeptAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeptAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeptAddLogic {
	return &DeptAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// -----------------------部门表-----------------------
func (l *DeptAddLogic) DeptAdd(in *pb.AddSysDeptReq) (*pb.AddSysDeptResp, error) {
	// 参数验证
	if in.Name == "" {
		l.Errorf("添加部门失败：部门名称不能为空")
		return nil, errorx.Msg("部门名称不能为空")
	}

	// 如果有父级部门，验证父级部门是否存在
	if in.ParentId > 0 {
		_, err := l.svcCtx.SysDept.FindOne(l.ctx, in.ParentId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("添加部门失败：父级部门不存在, parentId: %d", in.ParentId)
				return nil, errorx.Msg("父级部门不存在")
			}
			l.Errorf("查询父级部门失败: %v", err)
			return nil, errorx.Msg("查询父级部门失败")
		}
	}

	// 检查同级部门名称是否重复
	conditions := []string{"`parent_id` = ? AND `name` = ? AND"}
	args := []interface{}{in.ParentId, in.Name}
	query := conditions[0][:len(conditions[0])-4] // 去掉最后的 " AND"

	existingDepts, err := l.svcCtx.SysDept.SearchNoPage(l.ctx, "", true, query, args...)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("检查部门名称重复失败: %v", err)
		return nil, errorx.Msg("检查部门名称重复失败")
	}
	if len(existingDepts) > 0 {
		l.Errorf("添加部门失败：同级部门名称已存在, name: %s, parentId: %d", in.Name, in.ParentId)
		return nil, errorx.Msg("同级部门名称已存在")
	}

	// 构造部门数据
	dept := &model.SysDept{
		Name:      in.Name,
		ParentId:  in.ParentId,
		Remark:    in.Remark,
		Leader:    in.Leader,
		Phone:     in.Phone,
		Email:     in.Email,
		Status:    in.Status,
		Sort:      in.Sort,
		CreatedBy: in.CreatedBy,
		UpdatedBy: in.UpdatedBy,
	}

	// 保存到数据库
	_, err = l.svcCtx.SysDept.Insert(l.ctx, dept)
	if err != nil {
		l.Errorf("添加部门失败: %v", err)
		return nil, errorx.Msg("添加部门失败")
	}

	l.Infof("添加部门成功，部门名称: %s, 创建人: %s", in.Name, in.CreatedBy)
	return &pb.AddSysDeptResp{}, nil
}
