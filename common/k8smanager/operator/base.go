package operator

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"
)

// BaseOperator 基础操作器
// 提供通用的 Informer 缓存访问逻辑
type BaseOperator struct {
	// 日志器
	log logx.Logger
	ctx context.Context
	// 是否启用 Informer
	useInformer bool
}

// NewBaseOperator 创建基础操作器
func NewBaseOperator(ctx context.Context, useInformer bool) BaseOperator {
	return BaseOperator{
		ctx:         ctx,
		log:         logx.WithContext(ctx),
		useInformer: useInformer,
	}
}
