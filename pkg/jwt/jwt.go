package jwt

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
)

// 系统常量定义
const (
	BearerScheme           = "Bearer "          // Bearer认证方案前缀
	BearerSchemeLength     = len(BearerScheme)  // Bearer前缀长度
	DefaultAccessDuration  = 15 * time.Minute   // 默认访问令牌有效期
	DefaultRefreshDuration = 24 * time.Hour     // 默认刷新令牌有效期
	TokenIssuer            = "www.ikubeops.com" // 令牌发行者
)

// 自定义错误类型
var (
	ErrTokenEmpty         = errorx.New(100091, "令牌不能为空!")
	ErrInvalidToken       = errorx.New(100092, "无效的令牌!")
	ErrTokenExpired       = errorx.New(100093, "令牌已过期!")
	ErrTokenUsed          = errorx.New(100094, "令牌已被使用,请重新登陆!")
	ErrInvalidSigningAlgo = errorx.New(100095, "无效的签名算法!")
	ErrTokenMalformed     = errorx.New(100096, "令牌格式错误!")
	ErrTokenNotValidYet   = errorx.New(100097, "令牌尚未生效!")
	ErrTokenInvalidClaims = errorx.New(100098, "令牌声明无效!")
)

// jtiCache JTI缓存结构体，用于防止令牌重放攻击
type jtiCache struct {
	sync.RWMutex                      // 读写锁，保证并发安全
	usedJTIs     map[string]time.Time // 已使用的JTI映射表，key为JTI，value为使用时间
}

// 全局JTI缓存实例
var tokenCache = &jtiCache{
	usedJTIs: make(map[string]time.Time),
}

// AccountInfo 存放的账号信息
type AccountInfo struct {
	UserId   uint64   `json:"userId"`
	UserName string   `json:"userNmae"`
	Uuid     string   `json:"uuid"`
	NickName string   `json:"nickName"`
	Roles    []string `json:"roles"`
}

// JWTClaims 自定义的JWT声明结构体
type JWTClaims struct {
	UserName           *AccountInfo `json:"userName"` // 用户账号
	jwt.StandardClaims              // 标准JWT声明
}

// JWTResponse JWT响应结构体
type JWTResponse struct {
	AccessToken string `json:"accessToken"` // 访问令牌
	ExpiresAt   int64  `json:"expiresAt"`   // 过期时间
}

// VerifyToken 解析并验证JWT令牌
func VerifyToken(tokenString string, secretKey string) (*JWTClaims, errorx.ErrorX) {
	if tokenString == "" {
		return nil, ErrTokenEmpty
	}
	claims := &JWTClaims{}

	// 从Bearer字符串中提取JWT令牌
	jwtToken, err := extractTokenFromBearerString(tokenString)
	if err != nil {
		return nil, err
	}

	// 解析JWT令牌
	token, parseErr := jwt.ParseWithClaims(jwtToken, claims, func(token *jwt.Token) (interface{}, error) {
		// 验证签名算法是否为HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			// 返回自定义的签名算法错误
			return nil, ErrInvalidSigningAlgo
		}
		return []byte(secretKey), nil
	})

	if parseErr != nil {
		var ve *jwt.ValidationError
		if errors.As(parseErr, &ve) {
			switch {
			case ve.Errors&jwt.ValidationErrorMalformed != 0:
				// 令牌格式错误
				return nil, ErrTokenMalformed
			case ve.Errors&jwt.ValidationErrorExpired != 0:
				// 打印令牌事件 时间戳环卫 time.time
				// 令牌已过期
				return nil, ErrTokenExpired
			case ve.Errors&jwt.ValidationErrorNotValidYet != 0:
				// 令牌尚未生效
				return nil, ErrTokenNotValidYet
			case ve.Errors&jwt.ValidationErrorSignatureInvalid != 0:
				// 签名无效
				return nil, ErrInvalidSigningAlgo
			default:
				// 其他验证错误
				return nil, ErrInvalidToken
			}
		}
		// 非验证错误
		return nil, ErrInvalidToken
	}

	// 验证令牌有效性
	if !token.Valid {
		return nil, ErrInvalidToken
	}

	// 验证JTI是否已被使用
	//if err := checkAndMarkJTI(claims.Id); err != nil {
	//	return nil, err
	//}

	return claims, nil
}

// extractTokenFromBearerString 从Bearer认证字符串中提取JWT令牌
func extractTokenFromBearerString(bearerToken string) (string, errorx.ErrorX) {
	if len(bearerToken) < BearerSchemeLength || !strings.HasPrefix(bearerToken, BearerScheme) {
		return "", ErrInvalidToken
	}
	return bearerToken[BearerSchemeLength:], nil
}

// CreateJWTToken 生成访问令牌和刷新令牌
func CreateJWTToken(info *AccountInfo, secKey string, jtiUuid string, expirySeconds int64) (*JWTResponse, error) {
	// 计算令牌的签发时间，提前1分钟确保令牌的有效性时间
	now := time.Now()
	// 签发时间
	issuedAt := now.Add(-1 * time.Minute).Unix()
	// 过期时间
	accessExpiry := now.Add(time.Duration(expirySeconds) * time.Second).Unix()

	// 创建基础JWT声明
	baseClaims := JWTClaims{
		UserName: info,
		StandardClaims: jwt.StandardClaims{
			Audience:  TokenIssuer,
			Issuer:    TokenIssuer,   // 令牌发行者
			IssuedAt:  issuedAt,      // 发行时间
			NotBefore: issuedAt,      // 生效时间
			Id:        jtiUuid,       // 令牌唯一标识
			ExpiresAt: accessExpiry,  // 过期时间
			Subject:   info.UserName, // 令牌所有者
		},
	}

	// 生成访问令牌
	accessToken, err := signJWT(baseClaims, secKey)
	if err != nil {
		return nil, errorx.New(100098, fmt.Sprintf("生成访问令牌失败: %v", err))
	}

	return &JWTResponse{
		AccessToken: accessToken,
		ExpiresAt:   accessExpiry,
	}, nil
}

// signJWT 生成带签名的JWT令牌
func signJWT(claims JWTClaims, secKey string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secKey))
}

// checkAndMarkJTI 验证JTI是否已被使用（防止令牌重放）
func checkAndMarkJTI(jti string) errorx.ErrorX {
	tokenCache.Lock()
	defer tokenCache.Unlock()

	// 检查JTI是否已被使用
	if _, exists := tokenCache.usedJTIs[jti]; exists {
		return ErrTokenUsed
	}

	// 记录JTI使用时间
	tokenCache.usedJTIs[jti] = time.Now()

	// 清理过期的JTI记录
	cleanupExpiredJTIs()

	return nil
}

// cleanupExpiredJTIs 清理过期的JTI记录
func cleanupExpiredJTIs() {
	now := time.Now()
	for jti, timestamp := range tokenCache.usedJTIs {
		if now.Sub(timestamp) > DefaultRefreshDuration {
			delete(tokenCache.usedJTIs, jti)
		}
	}
}

// init 初始化，启动JTI清理的定时任务
//func init() {
//	go func() {
//		ticker := time.NewTicker(DefaultRefreshDuration)
//		defer ticker.Stop()
//		for {
//			<-ticker.C
//			cleanupExpiredJTIs()
//		}
//	}()
//}
