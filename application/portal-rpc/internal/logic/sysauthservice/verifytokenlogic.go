package sysauthservicelogic

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/code"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/common"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/vars"
	"github.com/yanshicheng/kube-nova/pkg/jwt"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type VerifyTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVerifyTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifyTokenLogic {
	return &VerifyTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 验证令牌
func (l *VerifyTokenLogic) VerifyToken(in *pb.VerifyTokenRequest) (*pb.VerifyTokenResponse, error) {
	jwtToken, err := l.verifyJWTToken(in.Token)
	if err != nil {
		tokenResp, matched, tokenErr := l.verifySysToken(in.Token)
		if tokenErr != nil {
			return nil, tokenErr
		}
		if matched {
			return tokenResp, nil
		}
		return &pb.VerifyTokenResponse{
			IsValid:      false,
			ErrorType:    int64(err.Code()),
			ErrorMessage: err.Message(),
		}, nil
	}
	redisKey := fmt.Sprintf("%s%s", common.UuidKeyPrefix, jwtToken.UserName.UserName)
	if common.AllowMultiLogin {
		redisKey = fmt.Sprintf("%s%s:%s", common.UuidKeyPrefix, jwtToken.UserName.UserName, jwtToken.UserName.Uuid)
	}
	key, errs := l.svcCtx.Cache.Get(redisKey)
	if errs != nil {
		l.Errorf("Redis 查询该 uuid 的访问令牌, 错误: %v, uuid: %v", err, redisKey)
		return nil, code.UUIDQueryErr
	}
	if key == "" {
		l.Infof("Redis 不存在该 uuid 的访问令牌, 错误: %v , key: %v", key, err)
		return &pb.VerifyTokenResponse{
			IsValid:      false,
			ErrorType:    100099,
			ErrorMessage: "token 已经被禁用，请联系管理员处理!",
		}, nil
	}

	if "Bearer "+key != in.Token {
		return &pb.VerifyTokenResponse{
			IsValid:      false,
			ErrorType:    int64(jwt.ErrTokenUsed.Code()),
			ErrorMessage: jwt.ErrTokenUsed.Message(),
		}, nil
	}

	return &pb.VerifyTokenResponse{
		IsValid:      true,
		ExpireTime:   jwtToken.ExpiresAt,
		Username:     jwtToken.UserName.UserName,
		UserId:       jwtToken.UserName.UserId,
		Roles:        jwtToken.UserName.Roles,
		NickName:     jwtToken.UserName.NickName,
		Uuid:         jwtToken.UserName.Uuid,
		ErrorType:    0,
		ErrorMessage: "",
	}, nil
}

func (l *VerifyTokenLogic) verifyJWTToken(token string) (*jwt.JWTClaims, errorx.ErrorX) {
	jwtToken, err := jwt.VerifyToken(token, vars.AccessSecret)
	if err == nil {
		return jwtToken, nil
	}
	configSecret := strings.TrimSpace(l.svcCtx.Config.AuthConfig.AccessSecret)
	if configSecret == "" || configSecret == vars.AccessSecret {
		return nil, err
	}
	if jwtToken, configErr := jwt.VerifyToken(token, configSecret); configErr == nil {
		return jwtToken, nil
	}
	return nil, err
}

func (l *VerifyTokenLogic) verifySysToken(authToken string) (*pb.VerifyTokenResponse, bool, error) {
	tokenValue := normalizeAuthToken(authToken)
	if tokenValue == "" {
		return nil, false, nil
	}
	token, err := l.svcCtx.SysToken.FindOneByTokenIsDeleted(l.ctx, tokenValue, 0)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, false, nil
		}
		l.Errorf("查询系统Token失败: %v", err)
		return nil, true, errorx.Msg("Token查询失败")
	}
	if token.Status != 1 {
		return invalidTokenResponse("Token已禁用"), true, nil
	}
	if token.ExpireTime.Valid && token.ExpireTime.Time.Before(time.Now()) {
		return &pb.VerifyTokenResponse{
			IsValid:      false,
			ErrorType:    int64(jwt.ErrTokenExpired.Code()),
			ErrorMessage: "Token已过期",
		}, true, nil
	}
	resp, err := l.sysTokenPrincipal(token)
	if err != nil {
		return nil, true, err
	}
	return resp, true, nil
}

func (l *VerifyTokenLogic) sysTokenPrincipal(token *model.SysToken) (*pb.VerifyTokenResponse, error) {
	expireTime := int64(0)
	if token.ExpireTime.Valid {
		expireTime = token.ExpireTime.Time.Unix()
	}
	uuid := fmt.Sprintf("sys-token:%d", token.Id)
	switch token.OwnerType {
	case 1:
		user, err := l.svcCtx.SysUser.FindOne(l.ctx, token.OwnerId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return invalidTokenResponse("Token所属用户不存在"), nil
			}
			l.Errorf("查询Token所属用户失败: %v", err)
			return nil, errorx.Msg("Token所属用户查询失败")
		}
		if user.Status == 0 {
			return invalidTokenResponse("Token所属用户已被冻结"), nil
		}
		roles, err := l.userRoleCodes(user.Id)
		if err != nil {
			return nil, err
		}
		return &pb.VerifyTokenResponse{
			IsValid:      true,
			ExpireTime:   expireTime,
			Username:     user.Username,
			UserId:       user.Id,
			Roles:        roles,
			NickName:     user.Nickname,
			Uuid:         uuid,
			ErrorType:    0,
			ErrorMessage: "",
		}, nil
	case 2:
		role, err := l.svcCtx.SysRole.FindOne(l.ctx, token.OwnerId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return invalidTokenResponse("Token所属角色不存在"), nil
			}
			l.Errorf("查询Token所属角色失败: %v", err)
			return nil, errorx.Msg("Token所属角色查询失败")
		}
		return &pb.VerifyTokenResponse{
			IsValid:      true,
			ExpireTime:   expireTime,
			Username:     token.Name,
			UserId:       0,
			Roles:        []string{role.Code},
			NickName:     token.Name,
			Uuid:         uuid,
			ErrorType:    0,
			ErrorMessage: "",
		}, nil
	default:
		return invalidTokenResponse("Token所有者类型无效"), nil
	}
}

func (l *VerifyTokenLogic) userRoleCodes(userId uint64) ([]string, error) {
	userRoles, err := l.svcCtx.SysUserRole.SearchNoPage(l.ctx, "id", false, "user_id = ?", userId)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("查询用户角色失败: %v", err)
		return nil, errorx.Msg("查询用户角色失败")
	}
	if len(userRoles) == 0 {
		return []string{"user", "R_SUPER"}, nil
	}
	roles := make([]string, 0, len(userRoles))
	for _, userRole := range userRoles {
		role, err := l.svcCtx.SysRole.FindOne(l.ctx, userRole.RoleId)
		if err != nil {
			continue
		}
		roles = append(roles, role.Code)
	}
	if len(roles) == 0 {
		return []string{"user", "R_SUPER"}, nil
	}
	return roles, nil
}

func invalidTokenResponse(message string) *pb.VerifyTokenResponse {
	return &pb.VerifyTokenResponse{
		IsValid:      false,
		ErrorType:    int64(jwt.ErrInvalidToken.Code()),
		ErrorMessage: message,
	}
}

func normalizeAuthToken(token string) string {
	token = strings.Trim(strings.TrimSpace(token), `"'`)
	if token == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(token), "authorization:") {
		token = strings.TrimSpace(token[len("authorization:"):])
	}
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	return strings.Trim(strings.TrimSpace(token), `"'`)
}
