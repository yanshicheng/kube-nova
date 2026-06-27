package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	portalpb "github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectDelLogic {
	return &ProjectDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectDel 删除项目（委托 portal-rpc）
func (l *ProjectDelLogic) ProjectDel(in *pb.DelOnecProjectReq) (*pb.DelOnecProjectResp, error) {
	l.Infof("开始删除项目，项目ID: %d", in.Id)

	// 检查项目下是否有集群配额（本地快速检查）
	clusters, err := l.svcCtx.OnecProjectClusterModel.SearchNoPage(l.ctx, "", true, "project_id = ?", in.Id)
	if err == nil && len(clusters) > 0 {
		l.Errorf("项目下存在集群配额，无法删除，项目ID: %d", in.Id)
		return nil, errorx.Msg("项目下存在集群配额，请先删除相关配置")
	}

	// 委托 portal-rpc 执行删除（portal 会做完整的依赖检查）
	_, err = l.svcCtx.PortalProjectRpc.DeleteProject(l.ctx, &portalpb.PortalDeleteProjectReq{
		Id:        in.Id,
		UpdatedBy: "manager-rpc",
	})
	if err != nil {
		l.Errorf("调用 portal 删除项目失败，ID: %d, 错误: %v", in.Id, err)
		return nil, err
	}

	l.Infof("删除项目成功，项目ID: %d", in.Id)
	return &pb.DelOnecProjectResp{}, nil
}
