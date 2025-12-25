package node

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteNodeAnnotationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteNodeAnnotationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteNodeAnnotationLogic {
	return &DeleteNodeAnnotationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteNodeAnnotationLogic) DeleteNodeAnnotation(req *types.NodeAnnotationDeleteRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}
	_, err = l.svcCtx.ManagerRpc.NodeDeleteAnnotations(l.ctx, &pb.ClusterNodeDeleteAnnotationsReq{
		Id:        req.Id,
		Key:       req.Key,
		UpdatedBy: username,
	})
	if err != nil {
		l.Errorf("删除节点注解失败: %v", err)
		return "", errorx.Msg("删除节点注解失败")
	}

	return "操作成功", nil
}
