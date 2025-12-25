package portalservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type APIGetByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAPIGetByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *APIGetByIdLogic {
	return &APIGetByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// APIGetById 根据ID获取API详细信息
func (l *APIGetByIdLogic) APIGetById(in *pb.GetSysAPIByIdReq) (*pb.GetSysAPIByIdResp, error) {
	// 参数验证
	if in.Id <= 0 {
		l.Errorf("查询API失败：API ID无效")
		return nil, errorx.Msg("API ID无效")
	}

	// 查询API信息
	api, err := l.svcCtx.SysApi.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("查询API失败：API不存在, apiId: %d", in.Id)
			return nil, errorx.Msg("API不存在")
		}
		l.Errorf("查询API信息失败: %v", err)
		return nil, errorx.Msg("查询API信息失败")
	}

	// 转换为响应格式
	pbAPI := l.convertToPbAPI(api)

	l.Infof("查询API详情成功，API ID: %d, API名称: %s, 路径: %s",
		in.Id, api.Name, api.Path)
	return &pb.GetSysAPIByIdResp{
		Data: pbAPI,
	}, nil
}

// convertToPbAPI 将数据库模型转换为protobuf格式
func (l *APIGetByIdLogic) convertToPbAPI(api *model.SysApi) *pb.SysAPI {
	return &pb.SysAPI{
		Id:           api.Id,
		ParentId:     api.ParentId,
		Name:         api.Name,
		Path:         api.Path,
		Method:       api.Method,
		IsPermission: api.IsPermission,
		CreatedBy:    api.CreatedBy,
		UpdatedBy:    api.UpdatedBy,
		CreatedAt:    api.CreatedAt.Unix(),
		UpdatedAt:    api.UpdatedAt.Unix(),
	}
}
