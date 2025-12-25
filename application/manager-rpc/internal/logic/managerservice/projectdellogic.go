package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

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

func (l *ProjectDelLogic) ProjectDel(in *pb.DelOnecProjectReq) (*pb.DelOnecProjectResp, error) {
	l.Logger.Infof("开始删除项目，项目ID: %d", in.Id)
	// 检查项目是否存在
	_, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("查询项目失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("项目不存在")
	}

	// 检查项目下是否有集群配额
	clusters, err := l.svcCtx.OnecProjectClusterModel.SearchNoPage(l.ctx, "", true, "project_id = ?", in.Id)
	if err == nil && len(clusters) > 0 {
		l.Logger.Error("项目下存在集群配额，无法删除，项目ID: %d", in.Id)
		return nil, errorx.Msg("项目下存在集群配额，请先删除相关配置")
	}

	// 执行软删除
	err = l.svcCtx.OnecProjectModel.DeleteSoft(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("删除项目失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("删除项目失败")
	}

	l.Logger.Infof("删除项目成功，项目ID: %d", in.Id)
	return &pb.DelOnecProjectResp{}, nil
}
