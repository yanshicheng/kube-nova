package managerservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectAdminGetByProjectIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectAdminGetByProjectIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectAdminGetByProjectIdLogic {
	return &ProjectAdminGetByProjectIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectAdminGetByProjectId 根据项目ID获取管理员列表
func (l *ProjectAdminGetByProjectIdLogic) ProjectAdminGetByProjectId(in *pb.GetOnecProjectAdminsByProjectIdReq) (*pb.GetOnecProjectAdminsByProjectIdResp, error) {
	// 参数校验
	if in.ProjectId == 0 {
		return nil, errorx.Msg("项目ID不能为空")
	}

	// 查询项目管理员列表
	queryStr := "`project_id` = ?"
	admins, err := l.svcCtx.OnecProjectAdminModel.SearchNoPage(l.ctx, "id", true, queryStr, in.ProjectId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			// 没有管理员也是正常情况，返回空列表
			l.Info("项目暂无管理员", "projectId", in.ProjectId)
			return &pb.GetOnecProjectAdminsByProjectIdResp{
				Data: []*pb.OnecProjectAdmin{},
			}, nil
		}
		l.Error("查询项目管理员失败", err)
		return nil, errorx.Msg("查询项目管理员失败")
	}

	// 转换数据结构
	var pbAdmins []*pb.OnecProjectAdmin
	for _, admin := range admins {
		pbAdmins = append(pbAdmins, &pb.OnecProjectAdmin{
			Id:        admin.Id,
			ProjectId: admin.ProjectId,
			UserId:    admin.UserId,
			CreatedAt: admin.CreatedAt.Unix(),
		})
	}

	return &pb.GetOnecProjectAdminsByProjectIdResp{
		Data: pbAdmins,
	}, nil
}
