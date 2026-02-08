package repositoryservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/model/repository"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListProjectRegistryBindingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListProjectRegistryBindingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProjectRegistryBindingsLogic {
	return &ListProjectRegistryBindingsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListProjectRegistryBindingsLogic) ListProjectRegistryBindings(in *pb.ListProjectRegistryBindingsReq) (*pb.ListProjectRegistryBindingsResp, error) {
	// 参数校验
	if in.RegistryId == 0 {
		return nil, errorx.Msg("仓库ID不能为空")
	}

	// 构建查询条件
	var conditions []string
	var args []interface{}

	// 必须条件：registry_id
	conditions = append(conditions, "`registry_id` = ?")
	args = append(args, in.RegistryId)

	// 可选条件：registry_project_name
	if in.RegistryProjectName != "" {
		conditions = append(conditions, "`registry_project_name` = ?")
		args = append(args, in.RegistryProjectName)
	}

	// 构建查询字符串
	queryStr := strings.Join(conditions, " AND ")

	// 查询绑定记录
	bindings, err := l.svcCtx.RegistryProjectBindingModel.SearchNoPage(
		l.ctx,
		"",
		true,
		queryStr,
		args...,
	)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		l.Errorf("查询项目仓库绑定失败: %v", err)
		return nil, errorx.Msg("查询项目仓库绑定失败")
	}

	var data []*pb.BindProjectIds
	projectIdSet := make(map[uint64]bool) // 使用 map 去重

	for _, binding := range bindings {
		l.Infof("项目仓库绑定: %+v", binding)
		// 去重：同一个项目只返回一次
		if !projectIdSet[binding.AppProjectId] {
			data = append(data, &pb.BindProjectIds{
				Id:        binding.Id,
				ProjectId: binding.AppProjectId,
			})
			projectIdSet[binding.AppProjectId] = true
		}
	}

	return &pb.ListProjectRegistryBindingsResp{
		Data: data,
	}, nil
}
