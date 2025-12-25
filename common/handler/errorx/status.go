package errorx

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"
	"github.com/yanshicheng/kube-nova/common/handler/errorx/types"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var _ ErrorX = (*Status)(nil)

type Status struct {
	sts *types.Status
}

func Error(code Code) *Status {
	return &Status{sts: &types.Status{Code: int32(code.Code()), Message: code.Message()}}
}

func Errorf(code Code, format string, args ...interface{}) *Status {
	code.msg = fmt.Sprintf(format, args...)
	return Error(code)
}
func (s *Status) Error() string {
	return s.Message()
}

func (s *Status) Code() int {
	return int(s.sts.Code)
}

func (s *Status) Message() string {
	if s.sts.Message == "" {
		return strconv.Itoa(int(s.sts.Code))
	}

	return s.sts.Message
}

func (s *Status) Details() []interface{} {
	if s == nil || s.sts == nil {
		return nil
	}
	details := make([]interface{}, 0, len(s.sts.Details))
	for _, d := range s.sts.Details {
		detail := &ptypes.DynamicAny{}

		if err := d.UnmarshalTo(detail); err != nil {
			details = append(details, err)
			continue
		}
		details = append(details, detail.Message)
	}

	return details
}

func (s *Status) WithDetails(msgs ...proto.Message) (*Status, error) {
	for _, msg := range msgs {
		anyMsg, err := anypb.New(msg)
		if err != nil {
			return s, err
		}
		s.sts.Details = append(s.sts.Details, anyMsg)
	}

	return s, nil
}

func (s *Status) Proto() *types.Status {
	return s.sts
}

// FromCode 根据自定义 Code 创建一个 Status 对象
func FromCode(code Code) *Status {
	return &Status{sts: &types.Status{Code: int32(code.Code()), Message: code.Message()}}
}

func FromProto(pbMsg proto.Message) ErrorX {
	msg, ok := pbMsg.(*types.Status)
	if ok {
		if len(msg.Message) == 0 || msg.Message == strconv.FormatInt(int64(msg.Code), 10) {
			return Code{code: int(msg.Code)}
		}
		return &Status{sts: msg}
	}

	return Errorf(ServerErr, "invalid proto message get %v", pbMsg)
}

func toErrorX(grpcStatus *status.Status) Code {
	grpcCode := grpcStatus.Code()
	switch grpcCode {
	case codes.OK:
		return OK
	case codes.InvalidArgument:
		return RequestErr
	case codes.NotFound:
		return NotFound
	case codes.PermissionDenied:
		return AccessDenied
	case codes.Unauthenticated:
		return Unauthorized
	case codes.ResourceExhausted:
		return ServerLimitExceed
	case codes.Unimplemented:
		return MethodNotAllowed
	case codes.DeadlineExceeded:
		return ServerDeadline
	case codes.Unavailable:
		return ServiceUnavailable
	case codes.Unknown:
		return String(grpcStatus.Message())
	}

	return ServerErr
}

// FromError 将标准 error 类型转换为 gRPC 的 status.Status 类型
func FromError(err error) *status.Status {
	err = errors.Cause(err)
	if code, ok := err.(ErrorX); ok {
		grpcStatus, e := gRPCStatusFromErrorX(code)
		if e == nil {
			return grpcStatus
		}
	}
	var grpcStatus *status.Status
	switch err {
	case context.Canceled:
		grpcStatus, _ = gRPCStatusFromErrorX(Canceled)
	case context.DeadlineExceeded:
		grpcStatus, _ = gRPCStatusFromErrorX(ServerDeadline)
	default:
		grpcStatus, _ = status.FromError(err)
	}

	return grpcStatus
}

// gRPCStatusFromErrorX 将实现了 ErrorX 接口的错误转换为 gRPC status.Status
func gRPCStatusFromErrorX(code ErrorX) (*status.Status, error) {
	var sts *Status
	switch v := code.(type) {
	case *Status:
		sts = v
	case Code:
		sts = FromCode(v)
	default:
		sts = Error(Code{code.Code(), code.Message()})
		for _, detail := range code.Details() {
			if msg, ok := detail.(proto.Message); ok {
				_, _ = sts.WithDetails(msg)
			}
		}
	}

	stas := status.New(codes.Code(int32(sts.Code())), sts.Message())
	return stas.WithDetails(sts.Proto())
}

func GrpcStatusToErrorX(gstatus *status.Status) ErrorX {
	details := gstatus.Details()
	for i := len(details) - 1; i >= 0; i-- {
		detail := details[i]
		if pb, ok := detail.(proto.Message); ok {
			return FromProto(pb)
		}
	}

	return toErrorX(gstatus)
}

func CodeFromError(err error) ErrorX {
	err = errors.Cause(err) // 获取原始错误

	// 判断是否是自定义错误类型
	if code, ok := err.(ErrorX); ok {
		return code
	}

	// 针对 context 相关错误
	switch err {
	case context.Canceled:
		return Canceled
	case context.DeadlineExceeded:
		return ServerDeadline
	}

	// 针对标准的 *errors.errorString 错误
	if reflect.TypeOf(err).String() == "*errors.errorString" {
		return Code{
			code: 50550,
			msg:  err.Error(),
		}
	}

	// 其他未知错误
	logx.Errorf("未知错误:%v", err)
	fmt.Printf("错误类型: %T\n", err)
	return ServerErr
}
