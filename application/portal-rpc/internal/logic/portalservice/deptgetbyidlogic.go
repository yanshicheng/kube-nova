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

type DeptGetByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeptGetByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeptGetByIdLogic {
	return &DeptGetByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DeptGetById 根据ID获取部门详细信息
func (l *DeptGetByIdLogic) DeptGetById(in *pb.GetSysDeptByIdReq) (*pb.GetSysDeptByIdResp, error) {

	// 参数验证
	if in.Id <= 0 {
		l.Errorf("查询部门失败：部门ID无效")
		return nil, errorx.Msg("部门ID无效")
	}

	// 查询部门信息
	dept, err := l.svcCtx.SysDept.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("查询部门失败：部门不存在, deptId: %d", in.Id)
			return nil, errorx.Msg("部门不存在")
		}
		l.Errorf("查询部门信息失败: %v", err)
		return nil, errorx.Msg("查询部门信息失败")
	}

	// 转换为响应格式
	pbDept := l.convertToPbDept(dept)

	return &pb.GetSysDeptByIdResp{
		Data: pbDept,
	}, nil
}

// convertToPbDept 将数据库模型转换为protobuf格式
func (l *DeptGetByIdLogic) convertToPbDept(dept *model.SysDept) *pb.SysDept {
	return &pb.SysDept{
		Id:        dept.Id,
		Name:      dept.Name,
		ParentId:  dept.ParentId,
		Remark:    dept.Remark,
		Leader:    dept.Leader,
		Phone:     dept.Phone,
		Email:     dept.Email,
		Status:    dept.Status,
		Sort:      dept.Sort,
		CreatedBy: dept.CreatedBy,
		UpdatedBy: dept.UpdatedBy,
		CreatedAt: dept.CreatedAt.Unix(),
		UpdatedAt: dept.UpdatedAt.Unix(),
	}
}
