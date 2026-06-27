package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectChannelDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectChannelDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectChannelDeleteLogic {
	return &ProjectChannelDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectChannelDeleteLogic) ProjectChannelDelete(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	binding, err := l.svcCtx.ProjectChannelModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("项目渠道删除失败: %v", err)
		return nil, err
	}
	if binding.IsDefault && isBuildGroupCode(l.ctx, l.svcCtx.ChannelGroupModel, resolveBindingGroupCode(l.ctx, l.svcCtx, binding)) {
		l.Errorf("默认构建渠道不能直接解绑，请先编辑其他构建渠道为默认")
		return nil, errorx.Msg("默认构建渠道不能直接解绑，请先编辑其他构建渠道为默认")
	}
	if err := l.svcCtx.ProjectChannelModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("项目渠道删除失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
