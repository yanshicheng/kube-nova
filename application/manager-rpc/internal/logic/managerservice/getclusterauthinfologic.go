package managerservicelogic

import (
	"context"
	"database/sql"
	"errors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterAuthInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetClusterAuthInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterAuthInfoLogic {
	return &GetClusterAuthInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 通过集群uuid获取集群认证信息
func (l *GetClusterAuthInfoLogic) GetClusterAuthInfo(in *pb.GetClusterAuthInfoReq) (*pb.GetClusterAuthInfoResp, error) {
	// 健康检查请求，直接返回错误，不打印日志
	if in.ClusterUuid == "__health_check__" {
		return nil, errorx.Msg("集群认证信息不存在")
	}

	//获取认证信息
	authInfo, err := l.svcCtx.OnecClusterAuthModel.FindOneByClusterUuid(l.ctx, in.ClusterUuid)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("集群认证信息不存在 [clusterUuid=%s]", in.ClusterUuid)
			return nil, errorx.Msg("集群认证信息不存在")
		}
		l.Errorf("查询集群认证信息失败: %v", err)
		return nil, errorx.Msg("查询集群认证信息失败")
	}
	return &pb.GetClusterAuthInfoResp{
		AuthType:           authInfo.AuthType,
		ApiServerHost:      authInfo.ApiServerHost,
		KubeFile:           getNullString(authInfo.Kubefile),
		Token:              authInfo.Token,
		CaCert:             getNullString(authInfo.CaCert),
		CaFile:             authInfo.CaFile,
		ClientCert:         getNullString(authInfo.ClientCert),
		CertFile:           authInfo.CertFile,
		ClientKey:          getNullString(authInfo.ClientKey),
		KeyFile:            authInfo.KeyFile,
		InsecureSkipVerify: authInfo.InsecureSkipVerify,
	}, nil

}

// 辅助函数
func getNullString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
