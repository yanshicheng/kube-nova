package errorx

import "strconv"

// ErrorX 接口定义了错误类型应具备的行为。
// 它包括获取错误字符串、错误码、错误消息和错误详情的方法。
type ErrorX interface {
	Error() string          // 返回错误的字符串表示
	Code() int              // 返回错误码
	Message() string        // 返回错误消息
	Details() []interface{} // 返回错误的详细信息
}

// Code 结构体实现了 ErrorX 接口，用于表示一个错误码及其关联的错误消息。
type Code struct {
	code int    // 错误码，使用整数表示
	msg  string // 错误消息，描述错误内容
}

// Error 方法实现了 ErrorX 接口中的 Error 方法。
// 如果 msg 字段不为空，返回错误消息；否则，返回错误码的字符串形式。
func (c Code) Error() string {
	if len(c.msg) > 0 {
		return c.msg
	}
	return strconv.Itoa(c.code) // 将整数的错误码转换为字符串并返回
}

// Code 方法实现了 ErrorX 接口中的 Code 方法。
// 返回 Code 结构体的 code 字段，即错误码。
func (c Code) Code() int {
	return c.code
}

// Message 方法实现了 ErrorX 接口中的 Message 方法。
// 它实际上调用了 Error 方法，因此返回与 Error 方法相同的内容。
func (c Code) Message() string {
	return c.Error()
}

// Details 方法实现了 ErrorX 接口中的 Details 方法。
// 当前实现返回 nil，表示没有附加的错误详情。
// 可以扩展此方法以返回更多与错误相关的详细信息。
func (c Code) Details() []interface{} {
	return nil
}

// String 函数接受一个字符串 s，并尝试将其转换为 Code 结构体。
// 如果字符串为空，则返回一个默认的 Code 对象（假设是 OK）。
// 如果字符串无法转换为整数（表示无效的错误码），则返回一个服务器错误的 Code 对象。
func String(s string) Code {
	if len(s) == 0 {
		return OK // 假设 OK 是一个已定义的全局变量，表示成功状态
	}
	code, err := strconv.Atoi(s) // 尝试将字符串转换为整数
	if err != nil {
		println("invalid code:", s)
		return ServerErr // 如果转换失败，返回服务器错误
	}

	return Code{code: code} // 返回包含转换后的错误码的 Code 对象
}

// New 函数创建并返回一个新的 Code 对象。
// 这个对象包含了指定的错误码和错误消息。
func New(code int, msg string) Code {
	return Code{code: code, msg: msg}
}

// add 函数与 New 函数类似，用于创建并返回一个新的 Code 对象。
// 它是一个辅助函数，可能用于内部初始化或简化代码。
func add(code int, msg string) Code {
	return Code{code: code, msg: msg}
}

// code 固定999999 msg 手动传递
func Msg(msg string) Code {
	return Code{code: 999999, msg: msg}
}
