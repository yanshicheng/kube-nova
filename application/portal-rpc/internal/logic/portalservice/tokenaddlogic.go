package portalservicelogic

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/pkg/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type TokenAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTokenAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TokenAddLogic {
	return &TokenAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// -----------------------API Token表-----------------------
// TokenAdd 添加API Token
func (l *TokenAddLogic) TokenAdd(in *pb.AddSysTokenReq) (*pb.AddSysTokenResp, error) {

	// 参数验证
	if in.OwnerType != 1 && in.OwnerType != 2 {
		l.Errorf("无效的所有者类型: %d", in.OwnerType)
		return nil, errorx.Msg("所有者类型无效，必须为1(用户)或2(角色)")
	}

	if in.OwnerId == 0 {
		l.Error("所有者ID不能为空")
		return nil, errorx.Msg("所有者ID不能为空")
	}

	if strings.TrimSpace(in.Name) == "" {
		l.Error("Token名称不能为空")
		return nil, errorx.Msg("Token名称不能为空")
	}

	if in.Type < 1 || in.Type > 3 {
		l.Errorf("无效的Token类型: %d", in.Type)
		return nil, errorx.Msg("Token类型无效，必须为1(临时)、2(长期)或3(永久)")
	}

	// 验证所有者是否存在
	if in.OwnerType == 1 {
		// 验证用户是否存在
		user, err := l.svcCtx.SysUser.FindOne(l.ctx, in.OwnerId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("用户不存在，用户ID: %d", in.OwnerId)
				return nil, errorx.Msg("指定的用户不存在")
			}
			l.Errorf("查询用户失败: %v", err)
			return nil, errorx.Msg("查询用户信息失败")
		}
		l.Infof("找到用户: %s", user.Username)
	} else if in.OwnerType == 2 {
		// 验证角色是否存在
		role, err := l.svcCtx.SysRole.FindOne(l.ctx, in.OwnerId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("角色不存在，角色ID: %d", in.OwnerId)
				return nil, errorx.Msg("指定的角色不存在")
			}
			l.Errorf("查询角色失败: %v", err)
			return nil, errorx.Msg("查询角色信息失败")
		}
		l.Infof("找到角色: %s", role.Name)
	}

	// 生成唯一的Token值
	tokenValue, err := utils.GenerateToken()
	if err != nil {
		l.Errorf("生成Token失败: %v", err)
		return nil, errorx.Msg("生成Token失败")
	}
	l.Infof("生成的Token值: %s", tokenValue)

	// 设置过期时间
	var expireTime sql.NullTime
	if in.Type == 1 {
		// 临时Token - 7天有效期
		expireTime = sql.NullTime{
			Time:  time.Now().Add(7 * 24 * time.Hour),
			Valid: true,
		}
		l.Info("设置临时Token过期时间为7天后")
	} else if in.Type == 2 {
		// 长期Token - 90天有效期
		expireTime = sql.NullTime{
			Time:  time.Now().Add(90 * 24 * time.Hour),
			Valid: true,
		}
		l.Info("设置长期Token过期时间为90天后")
	} else if in.Type == 3 {
		// 永久Token - 不设置过期时间
		expireTime = sql.NullTime{Valid: false}
		l.Info("设置永久Token，不过期")
	}

	// 处理过期时间参数（如果用户指定了过期时间，使用用户指定的）
	if in.ExpireTime > 0 {
		expireTime = sql.NullTime{
			Time:  time.Unix(in.ExpireTime, 0),
			Valid: true,
		}
		l.Infof("使用用户指定的过期时间: %s", expireTime.Time.Format("2006-01-02 15:04:05"))
	}

	// 构建Token数据
	tokenData := &model.SysToken{
		OwnerType:  in.OwnerType,
		OwnerId:    in.OwnerId,
		Token:      tokenValue,
		Name:       in.Name,
		Type:       in.Type,
		ExpireTime: expireTime,
		Status:     in.Status,
		CreatedBy:  in.CreatedBy,
		UpdatedBy:  in.UpdatedBy,
	}

	// 插入数据库
	result, err := l.svcCtx.SysToken.Insert(l.ctx, tokenData)
	if err != nil {
		l.Errorf("插入Token失败: %v", err)
		return nil, errorx.Msg("创建Token失败")
	}

	// 获取插入的ID
	tokenId, err := result.LastInsertId()
	if err != nil {
		l.Errorf("获取Token ID失败: %v", err)
		return nil, errorx.Msg("获取Token ID失败")
	}

	l.Infof("成功创建Token，ID: %d, Token值: %s", tokenId, tokenValue)

	return &pb.AddSysTokenResp{
		Token: tokenValue,
	}, nil
}
