package repositoryservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/model/repository"
	repository2 "github.com/yanshicheng/kube-nova/application/console-rpc/internal/model/repository"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type BindRegistryProjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBindRegistryProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindRegistryProjectLogic {
	return &BindRegistryProjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ============ 仓库项目与应用项目绑定（管理员操作）============
func (l *BindRegistryProjectLogic) BindRegistryProject(in *pb.BindRegistryProjectReq) (*pb.BindRegistryProjectResp, error) {
	// 检查 registry 是否存在
	_, err := l.svcCtx.ContainerRegistryModel.FindOne(l.ctx, in.RegistryId)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errorx.Msg("仓库集群关联不存在")
		}
		return nil, errorx.Msg("查询仓库集群关联失败")
	}

	// 检查是否已绑定
	existing, _ := l.svcCtx.RegistryProjectBindingModel.FindOneByRegistryIdAppProjectIdRegistryProjectNameIsDeleted(
		l.ctx,
		in.RegistryId,
		in.AppProjectId,
		in.RegistryProjectName,
		0,
	)
	if existing != nil {
		return nil, errorx.Msg("该仓库项目已绑定到此应用项目")
	}

	// 创建绑定
	data := &repository2.RegistryProjectBinding{
		RegistryId:          in.RegistryId,
		AppProjectId:        in.AppProjectId,
		RegistryProjectName: in.RegistryProjectName,
		RegistryProjectId:   in.RegistryProjectId,
		IsDeleted:           0,
	}

	result, err := l.svcCtx.RegistryProjectBindingModel.Insert(l.ctx, data)
	if err != nil {
		return nil, errorx.Msg("绑定仓库项目失败")
	}

	id, _ := result.LastInsertId()
	return &pb.BindRegistryProjectResp{
		Id:      uint64(id),
		Message: "绑定成功",
	}, nil
}
