package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectAllSyncLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectAllSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectAllSyncLogic {
	return &ProjectAllSyncLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 同步所有项目配额
func (l *ProjectAllSyncLogic) ProjectAllSync(in *pb.ProjectQuotaSyncReq) (*pb.ProjectQuotaSyncResp, error) {
	// 获取所有项目
	_, err := l.svcCtx.OnecProjectModel.SearchNoPage(l.ctx, "", true, "", nil)
	if err != nil {
		l.Errorf("获取所有项目失败: %v", err)
		return nil, errorx.Msg("获取所有项目失败")
	}
	err = l.svcCtx.SyncOperator.SyncAll(l.ctx, in.Operator, true)
	if err != nil {
		l.Errorf("同步所有项目配额失败: %v", err)
		return nil, errorx.Msg("同步所有项目配额失败")
	}
	return &pb.ProjectQuotaSyncResp{}, nil
}
