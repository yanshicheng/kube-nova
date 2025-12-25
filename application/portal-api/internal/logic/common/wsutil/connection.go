// internal/common/wsutil/connection.go
package wsutil

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zeromicro/go-zero/core/logx"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  8192,
	WriteBufferSize: 8192,
	CheckOrigin: func(r *http.Request) bool {
		return true // 生产环境需要严格验证
	},
}

// ==================== 消息类型定义 ====================
const (
	TypeSuccess = "success"
	TypeError   = "error"
	TypePing    = "ping"
	TypePong    = "pong"

	TypeInitial    = "initial"     // 初始数据
	TypeNewMessage = "new_message" // 新消息推送

	TypeLogInit  = "log_init"
	TypeLogData  = "log_data"
	TypeLogEnd   = "log_end"
	TypeLogError = "log_error"

	TypeExecInit   = "exec_init"
	TypeExecStdin  = "exec_stdin"
	TypeExecStdout = "exec_stdout"
	TypeExecStderr = "exec_stderr"
	TypeExecResize = "exec_resize"
	TypeExecExit   = "exec_exit"

	TypeFileTailInit = "tail_init"
	TypeFileTailData = "tail_data"
	TypeFileTailEnd  = "tail_end"
)

// WSMessage WebSocket 消息结构
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// ErrorMessage 错误消息
type ErrorMessage struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

// ==================== WebSocket 连接封装 ====================

type WSConnection struct {
	conn         *websocket.Conn
	mu           sync.Mutex
	writeMu      sync.Mutex
	closeChan    chan struct{}
	closeOnce    sync.Once
	writeChan    chan []byte
	closed       atomic.Bool
	clientClosed atomic.Bool
	lastWrite    atomic.Int64
	lastRead     atomic.Int64
	lastPong     atomic.Int64
	missedPings  atomic.Int32
}

// UpgradeWebSocket 升级 HTTP 连接为 WebSocket
func UpgradeWebSocket(w http.ResponseWriter, r *http.Request) (*WSConnection, error) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	ws := &WSConnection{
		conn:      conn,
		closeChan: make(chan struct{}),
		writeChan: make(chan []byte, 256),
	}
	ws.closed.Store(false)
	ws.clientClosed.Store(false)
	ws.lastWrite.Store(now)
	ws.lastRead.Store(now)
	ws.lastPong.Store(now)
	ws.missedPings.Store(0)

	// 设置 Pong 处理器
	conn.SetPongHandler(func(string) error {
		ws.lastPong.Store(time.Now().Unix())
		ws.missedPings.Store(0)
		err := ws.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		if err != nil {
			return err
		}
		logx.Debugf("收到客户端 pong，重置未响应计数")
		return nil
	})

	// 设置 Close 处理器
	conn.SetCloseHandler(func(code int, text string) error {
		logx.Infof("收到客户端关闭帧: code=%d, text=%s", code, text)
		ws.clientClosed.Store(true)
		err := ws.Close()
		if err != nil {
			return err
		}
		return nil
	})

	// 启动写入循环
	go ws.writeLoop()

	return ws, nil
}

// writeLoop 写入循环
func (c *WSConnection) writeLoop() {
	defer func() {
		if r := recover(); r != nil {
			logx.Errorf("WebSocket writeLoop panic: %v", r)
		}
	}()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case data, ok := <-c.writeChan:
			if !ok {
				return
			}
			if err := c.writeMessage(websocket.TextMessage, data); err != nil {
				if !c.IsClosed() && !c.IsClientClosed() {
					logx.Errorf("WebSocket 写入错误: %v", err)
				}
				c.Close()
				return
			}

		case <-ticker.C:
			lastWrite := c.lastWrite.Load()
			if time.Now().Unix()-lastWrite > 30 {
				if !c.IsConnectionAlive() {
					logx.Info("WebSocket 写入超时且连接不活跃，关闭连接")
					err := c.Close()
					if err != nil {
						return
					}
					return
				}
			}

		case <-c.closeChan:
			return
		}
	}
}

// writeMessage 底层写入方法
func (c *WSConnection) writeMessage(messageType int, data []byte) error {
	if c.IsClosed() {
		return websocket.ErrCloseSent
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if err := c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}

	if err := c.conn.WriteMessage(messageType, data); err != nil {
		return err
	}

	c.lastWrite.Store(time.Now().Unix())
	return nil
}

// WriteJSON 发送 JSON 消息
func (c *WSConnection) WriteJSON(msg interface{}) error {
	if c.IsClosed() {
		return websocket.ErrCloseSent
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case <-c.closeChan:
		return websocket.ErrCloseSent
	case c.writeChan <- data:
		return nil
	case <-time.After(5 * time.Second):
		logx.Error("WebSocket 写入通道超时")
		return errors.New("write timeout")
	}
}

// WriteMessage 发送原始消息
func (c *WSConnection) WriteMessage(messageType int, data []byte) error {
	return c.writeMessage(messageType, data)
}

// ReadJSON 读取 JSON 消息
func (c *WSConnection) ReadJSON(v interface{}) error {
	if c.IsClosed() {
		return websocket.ErrCloseSent
	}

	if err := c.conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
		return err
	}

	err := c.conn.ReadJSON(v)
	if err == nil {
		c.lastRead.Store(time.Now().Unix())
		return nil
	}

	// 检测客户端关闭
	if websocket.IsCloseError(err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseAbnormalClosure,
	) {
		logx.Infof("客户端关闭连接: %v", err)
		c.clientClosed.Store(true)
		c.Close()
		return err
	}

	if websocket.IsUnexpectedCloseError(err) {
		logx.Errorf("WebSocket 异常关闭: %v", err)
		c.clientClosed.Store(true)
		c.Close()
		return err
	}

	return err
}

// ReadMessage 读取原始消息
func (c *WSConnection) ReadMessage() (messageType int, p []byte, err error) {
	if c.IsClosed() {
		return 0, nil, websocket.ErrCloseSent
	}

	if err := c.conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
		return 0, nil, err
	}

	messageType, p, err = c.conn.ReadMessage()
	if err == nil {
		c.lastRead.Store(time.Now().Unix())
		return
	}

	if websocket.IsCloseError(err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseAbnormalClosure,
	) {
		logx.Infof("客户端关闭连接: %v", err)
		c.clientClosed.Store(true)
		c.Close()
		return
	}

	if websocket.IsUnexpectedCloseError(err) {
		logx.Errorf("WebSocket 异常关闭: %v", err)
		c.clientClosed.Store(true)
		c.Close()
		return
	}

	return
}

// Close 关闭连接
func (c *WSConnection) Close() error {
	c.closeOnce.Do(func() {
		if !c.clientClosed.Load() {
			logx.Info("服务端关闭 WebSocket 连接")
			c.closed.Store(true)

			c.writeMu.Lock()
			err := c.conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "server closing"),
				time.Now().Add(time.Second),
			)
			if err != nil {
				return
			}
			c.writeMu.Unlock()
		} else {
			logx.Info("客户端已关闭，服务端清理")
			c.closed.Store(true)
		}

		close(c.closeChan)
		close(c.writeChan)

		time.Sleep(100 * time.Millisecond)

		c.mu.Lock()
		err := c.conn.Close()
		if err != nil {
			return
		}
		c.mu.Unlock()

		logx.Info("WebSocket 连接已关闭")
	})
	return nil
}

// IsClosed 检查连接是否已关闭
func (c *WSConnection) IsClosed() bool {
	return c.closed.Load()
}

// IsClientClosed 检查是否是客户端主动关闭
func (c *WSConnection) IsClientClosed() bool {
	return c.clientClosed.Load()
}

// CloseChan 获取关闭通道
func (c *WSConnection) CloseChan() <-chan struct{} {
	return c.closeChan
}

// SetReadDeadline 设置读取超时
func (c *WSConnection) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// StartPingPong 启动心跳检测
func (c *WSConnection) StartPingPong(interval time.Duration) {
	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()
		defer func() {
			if r := recover(); r != nil {
				logx.Errorf("心跳检测 panic: %v", r)
			}
		}()

		for {
			select {
			case <-ticker.C:
				if c.IsClosed() {
					return
				}

				if c.IsClientClosed() {
					logx.Debug("客户端已关闭，停止心跳")
					return
				}

				missed := c.missedPings.Load()
				if missed >= 3 {
					logx.Errorf("客户端未响应心跳，已丢失 %d 次，关闭连接", missed)
					c.Close()
					return
				}

				if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
					if !c.IsClosed() && !c.IsClientClosed() {
						logx.Errorf("发送心跳失败: %v", err)
					}
					err := c.Close()
					if err != nil {
						return
					}
					return
				}

				c.missedPings.Add(1)
				logx.Debugf("已发送心跳，未响应次数: %d/3", c.missedPings.Load())

			case <-c.closeChan:
				return
			}
		}
	}()
}

// IsConnectionAlive 检查连接是否活跃
func (c *WSConnection) IsConnectionAlive() bool {
	if c.IsClosed() {
		return false
	}

	if c.IsClientClosed() {
		return false
	}

	lastPong := c.lastPong.Load()
	if time.Now().Unix()-lastPong > 90 {
		logx.Debugf("90秒内未收到 pong 响应")
		return false
	}

	return true
}

// ==================== 便捷方法 ====================

func (c *WSConnection) SendMessage(msgType string, data interface{}) error {
	return c.WriteJSON(WSMessage{
		Type: msgType,
		Data: data,
	})
}

func (c *WSConnection) SendError(err error) error {
	if c.IsClientClosed() {
		return nil
	}
	return c.SendMessage(TypeError, ErrorMessage{
		Message: err.Error(),
	})
}

func (c *WSConnection) SendErrorWithCode(code string, message string) error {
	if c.IsClientClosed() {
		return nil
	}
	return c.SendMessage(TypeError, ErrorMessage{
		Code:    code,
		Message: message,
	})
}

func (c *WSConnection) SendSuccess(data interface{}) error {
	return c.SendMessage(TypeSuccess, data)
}

func ParseMessage(data []byte) (*WSMessage, error) {
	var msg WSMessage
	err := json.Unmarshal(data, &msg)
	return &msg, err
}
