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

type ClusterAuthInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClusterAuthInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterAuthInfoLogic {
	return &ClusterAuthInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ClusterAuthInfoLogic) ClusterAuthInfo(in *pb.ClusterAuthInfoReq) (*pb.ClusterAuthInfoResp, error) {
	// 1. 查询认证信息
	auth, err := l.svcCtx.OnecClusterAuthModel.FindOneByClusterUuid(l.ctx, in.ClusterUuid)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("集群认证信息不存在 [clusterUuid=%s]", in.ClusterUuid)
			return nil, errorx.Msg("集群认证信息不存在")
		}
		l.Errorf("查询集群认证信息失败: %v", err)
		return nil, errorx.Msg("查询集群认证信息失败")
	}

	// 2. 构建响应（注意处理可能的空值）
	resp := &pb.ClusterAuthInfoResp{
		Id:                 auth.Id,
		ClusterUuid:        auth.ClusterUuid,
		AuthType:           auth.AuthType,
		ApiServerHost:      auth.ApiServerHost,
		Token:              auth.Token,
		CaFile:             auth.CaFile,
		CertFile:           auth.CertFile,
		KeyFile:            auth.KeyFile,
		InsecureSkipVerify: auth.InsecureSkipVerify,
		CreatedAt:          auth.CreatedAt.Unix(),
		UpdatedAt:          auth.UpdatedAt.Unix(),
	}

	// 处理可能的 NULL 值
	if auth.Kubefile.Valid {
		resp.KeyFile = getNullString(auth.Kubefile)
	}
	if auth.CaCert.Valid {
		resp.CaCert = getNullString(auth.CaCert)
	}
	if auth.ClientCert.Valid {
		resp.ClientCert = getNullString(auth.ClientCert)
	}
	if auth.ClientKey.Valid {
		resp.ClientKey = getNullString(auth.ClientKey)
	}

	return resp, nil
}
