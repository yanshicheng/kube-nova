package portalservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/vars"

	"github.com/zeromicro/go-zero/core/logx"
)

type TokenSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTokenSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TokenSearchLogic {
	return &TokenSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// TokenSearch 搜索Token
func (l *TokenSearchLogic) TokenSearch(in *pb.SearchSysTokenReq) (*pb.SearchSysTokenResp, error) {
	// 记录请求日志
	l.Infof("开始搜索Token，页码: %d, 每页数量: %d", in.Page, in.PageSize)
	// 设置默认值
	if in.Page == 0 {
		in.Page = vars.Page
	}
	if in.PageSize == 0 {
		in.PageSize = vars.PageSize
	}
	if in.OrderField == "" {
		in.OrderField = vars.OrderField
	}

	isAsc := in.IsAsc

	// 构建查询条件
	var conditions []string
	var args []interface{}

	// 根据所有者类型查询
	if in.OwnerType > 0 {
		conditions = append(conditions, "`owner_type` = ?")
		args = append(args, in.OwnerType)
		l.Infof("添加查询条件: owner_type = %d", in.OwnerType)
	}

	// 根据所有者ID查询
	if in.OwnerId > 0 {
		conditions = append(conditions, "`owner_id` = ?")
		args = append(args, in.OwnerId)
		l.Infof("添加查询条件: owner_id = %d", in.OwnerId)
	}

	// 根据Token值模糊查询
	if in.Token != "" {
		conditions = append(conditions, "`token` LIKE ?")
		args = append(args, "%"+in.Token+"%")
		l.Infof("添加查询条件: token LIKE '%%%s%%'", in.Token)
	}

	// 根据名称模糊查询
	if in.Name != "" {
		conditions = append(conditions, "`name` LIKE ?")
		args = append(args, "%"+in.Name+"%")
		l.Infof("添加查询条件: name LIKE '%%%s%%'", in.Name)
	}

	// 根据类型查询
	if in.Type > 0 {
		conditions = append(conditions, "`type` = ?")
		args = append(args, in.Type)
		l.Infof("添加查询条件: type = %d", in.Type)
	}

	// 拼接查询条件
	queryStr := ""
	if len(conditions) > 0 {
		queryStr = strings.Join(conditions, " AND ")
	}

	// 执行搜索
	tokens, total, err := l.svcCtx.SysToken.Search(
		l.ctx,
		in.OrderField,
		isAsc,
		in.Page,
		in.PageSize,
		queryStr,
		args...,
	)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Info("未找到匹配的Token记录")
			return &pb.SearchSysTokenResp{
				Data:  []*pb.SysToken{},
				Total: 0,
			}, nil
		}
		l.Errorf("搜索Token失败: %v", err)
		return nil, errorx.Msg("搜索Token失败")
	}

	l.Infof("搜索到%d条Token记录", total)

	// 转换响应数据
	var respData []*pb.SysToken
	for _, token := range tokens {
		var expireTime int64
		if token.ExpireTime.Valid {
			expireTime = token.ExpireTime.Time.Unix()
		}

		respData = append(respData, &pb.SysToken{
			Id:         token.Id,
			OwnerType:  token.OwnerType,
			OwnerId:    token.OwnerId,
			Token:      token.Token,
			Name:       token.Name,
			Type:       token.Type,
			ExpireTime: expireTime,
			Status:     token.Status,
			CreatedBy:  token.CreatedBy,
			UpdatedBy:  token.UpdatedBy,
			CreatedAt:  token.CreatedAt.Unix(),
			UpdatedAt:  token.UpdatedAt.Unix(),
		})
	}

	return &pb.SearchSysTokenResp{
		Data:  respData,
		Total: total,
	}, nil
}
