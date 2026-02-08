package portalservicelogic

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserGetByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUserGetByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserGetByIdLogic {
	return &UserGetByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UserGetById 根据ID获取用户信息
func (l *UserGetByIdLogic) UserGetById(in *pb.GetSysUserByIdReq) (*pb.GetSysUserByIdResp, error) {
	// 参数验证
	if in.Id == 0 {
		l.Error("用户ID不能为空")
		return nil, errorx.Msg("用户ID不能为空")
	}

	// 查询用户信息
	user, err := l.svcCtx.SysUser.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("查询用户失败，用户ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("用户不存在")
	}
	absIcon := fmt.Sprintf("%s/%s%s", l.svcCtx.Config.StorageConf.EndpointProxy, l.svcCtx.Config.StorageConf.BucketName, user.Avatar)
	// 转换为protobuf格式
	pbUser := &pb.SysUser{
		Id:             user.Id,
		Username:       user.Username,
		Nickname:       user.Nickname,
		Avatar:         absIcon,
		Email:          user.Email,
		Phone:          user.Phone,
		WorkNumber:     user.WorkNumber,
		DeptId:         user.DeptId,
		Status:         user.Status,
		IsNeedResetPwd: user.IsNeedResetPwd,
		CreatedBy:      user.CreatedBy,
		UpdatedBy:      user.UpdatedBy,
		CreatedAt:      user.CreatedAt.Unix(),
		UpdatedAt:      user.UpdatedAt.Unix(),
		DingtalkId:     user.DingtalkId.String,
		WechatId:       user.WechatId.String,
		FeishuId:       user.FeishuId.String,
	}

	return &pb.GetSysUserByIdResp{Data: pbUser}, nil
}
