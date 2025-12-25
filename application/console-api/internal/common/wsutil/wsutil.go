// ========== 文件: wsutil/wsutil.go ==========

package wsutil

import (
	"context"
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
		return true
	},
}

// ==================== 消息类型定义 ====================

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

const (
	TypeSuccess = "success"
	TypeError   = "error"
	TypePing    = "ping"
	TypePong    = "pong"

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

type LogStreamMessage struct {
	Timestamp string `json:"timestamp,omitempty"`
	Log       string `json:"log"`
	Stream    string `json:"stream"`
	Container string `json:"container,omitempty"`
}

type ExecResizeMessage struct {
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

type ExecStdinMessage struct {
	Data string `json:"data"`
}

type ExecOutputMessage struct {
	Data   string `json:"data"`
	Stream string `json:"stream"`
}

type FileTailMessage struct {
	Line       string `json:"line"`
	LineNumber int    `json:"lineNumber"`
	Timestamp  int64  `json:"timestamp"`
}

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
	lastWrite    atomic.Int64
	lastRead     atomic.Int64
	lastPong     atomic.Int64
	missedPings  atomic.Int32
	clientClosed atomic.Bool
}

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

	conn.SetPongHandler(func(string) error {
		ws.lastPong.Store(time.Now().Unix())
		ws.missedPings.Store(0)
		ws.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		logx.Debugf("received pong from client, reset missed pings")
		return nil
	})

	conn.SetCloseHandler(func(code int, text string) error {
		logx.Infof("received close frame from client: code=%d, text=%s", code, text)
		ws.clientClosed.Store(true) // 标记客户端主动关闭
		ws.Close()
		return nil
	})

	go ws.writeLoop()

	return ws, nil
}

// writeLoop 写入循环（避免并发写入冲突）
func (c *WSConnection) writeLoop() {
	defer func() {
		if r := recover(); r != nil {
			logx.Errorf("ws writeLoop panic recovered: %v", r)
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
					logx.Errorf("ws write error: %v", err)
				}
				c.Close()
				return
			}

		case <-ticker.C:
			lastWrite := c.lastWrite.Load()
			if time.Now().Unix()-lastWrite > 30 {
				if !c.IsConnectionAlive() {
					logx.Info("ws write timeout and connection not alive, closing")
					c.Close()
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

	// 设置写入超时
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
		logx.Error("ws write channel timeout")
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

	// 设置读取超时
	if err := c.conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
		return err
	}

	err := c.conn.ReadJSON(v)
	if err == nil {
		c.lastRead.Store(time.Now().Unix())
		return nil
	}

	if websocket.IsCloseError(err,
		websocket.CloseNormalClosure,   // 1000: 正常关闭
		websocket.CloseGoingAway,       // 1001: 客户端离开
		websocket.CloseAbnormalClosure, // 1006: 异常关闭
	) {
		logx.Infof("websocket closed by client: %v", err)
		c.clientClosed.Store(true) // 标记客户端主动关闭
		c.Close()
		return err
	}

	if websocket.IsUnexpectedCloseError(err) {
		logx.Errorf("unexpected websocket close: %v", err)
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
		logx.Infof("websocket closed by client: %v", err)
		c.clientClosed.Store(true)
		c.Close()
		return
	}

	if websocket.IsUnexpectedCloseError(err) {
		logx.Errorf("unexpected websocket close: %v", err)
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
			logx.Info("server closing websocket connection")

			c.closed.Store(true)

			c.writeMu.Lock()
			c.conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "server closing"),
				time.Now().Add(time.Second),
			)
			c.writeMu.Unlock()
		} else {
			logx.Info("client closed websocket, server cleanup")
			c.closed.Store(true)
		}

		close(c.closeChan)
		close(c.writeChan)

		time.Sleep(100 * time.Millisecond)

		c.mu.Lock()
		c.conn.Close()
		c.mu.Unlock()

		logx.Info("websocket connection closed")
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
				logx.Errorf("ws pingpong panic recovered: %v", r)
			}
		}()

		for {
			select {
			case <-ticker.C:
				if c.IsClosed() {
					return
				}

				if c.IsClientClosed() {
					logx.Debug("client already closed, stop ping")
					return
				}

				missed := c.missedPings.Load()
				if missed >= 3 {
					logx.Errorf("client not responding to ping, missed %d times, closing connection", missed)
					c.Close()
					return
				}

				// 发送 ping
				if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
					if !c.IsClosed() && !c.IsClientClosed() {
						logx.Errorf("ping failed: %v", err)
					}
					c.Close()
					return
				}

				// 增加未响应计数
				c.missedPings.Add(1)
				logx.Debugf("sent ping, missed count: %d/%d", c.missedPings.Load(), 3)

			case <-c.closeChan:
				return
			}
		}
	}()
}

// IsConnectionAlive 检查连接是否真的活跃
func (c *WSConnection) IsConnectionAlive() bool {
	if c.IsClosed() {
		return false
	}

	if c.IsClientClosed() {
		return false
	}

	// 检查最近是否收到过 pong（90秒内）
	lastPong := c.lastPong.Load()
	if time.Now().Unix()-lastPong > 90 {
		logx.Debugf("no pong received in last 90 seconds")
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

func (c *WSConnection) SendLog(log string, stream string, timestamp string, container string) error {
	return c.SendMessage(TypeLogData, LogStreamMessage{
		Log:       log,
		Stream:    stream,
		Timestamp: timestamp,
		Container: container,
	})
}

func (c *WSConnection) SendExecOutput(data string, stream string) error {
	msgType := TypeExecStdout
	if stream == "stderr" {
		msgType = TypeExecStderr
	}
	return c.SendMessage(msgType, ExecOutputMessage{
		Data:   data,
		Stream: stream,
	})
}

func ParseMessage(data []byte) (*WSMessage, error) {
	var msg WSMessage
	err := json.Unmarshal(data, &msg)
	return &msg, err
}

// ==================== 辅助函数 ====================

// WithTimeout 为函数调用添加超时控制
func WithTimeout(ctx context.Context, timeout time.Duration, fn func() error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- fn()
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
