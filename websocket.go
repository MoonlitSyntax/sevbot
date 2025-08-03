package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketConfig WebSocket连接配置
type WebSocketConfig struct {
	URL         string            `json:"url"`
	AccessToken string            `json:"access_token"`
	Headers     map[string]string `json:"headers"`

	// 超时配置
	ConnectTimeout  time.Duration `json:"connect_timeout"`
	ReadTimeout     time.Duration `json:"read_timeout"`
	WriteTimeout    time.Duration `json:"write_timeout"`
	ResponseTimeout time.Duration `json:"response_timeout"`

	// 重连配置
	AutoReconnect  bool          `json:"auto_reconnect"`
	ReconnectDelay time.Duration `json:"reconnect_delay"`

	// 心跳配置
	PingInterval time.Duration `json:"ping_interval"`
}

// DefaultWebSocketConfig 返回默认配置
func DefaultWebSocketConfig(url string) WebSocketConfig {
	return WebSocketConfig{
		URL:             url,
		ConnectTimeout:  10 * time.Second,
		ReadTimeout:     60 * time.Second,
		WriteTimeout:    10 * time.Second,
		ResponseTimeout: 30 * time.Second,
		AutoReconnect:   true,
		ReconnectDelay:  5 * time.Second,
		PingInterval:    30 * time.Second,
		Headers:         make(map[string]string),
	}
}

// WebSocketAdapter WebSocket适配器
type WebSocketAdapter struct {
	config  WebSocketConfig
	conn    *websocket.Conn
	eventCh chan []byte
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.RWMutex
	logger  *slog.Logger

	// 状态
	connected      atomic.Bool
	messagesSent   atomic.Int64
	messagesRecv   atomic.Int64
	reconnectCount atomic.Int64

	// API调用管理
	pendingCalls sync.Map // map[string]chan *APIResponse
	callID       atomic.Int64

	// 心跳管理
	lastHeartbeat  time.Time
	heartbeatTimer *time.Timer
	heartbeatMu    sync.RWMutex
}

// NewWebSocketAdapter 创建WebSocket适配器
func NewWebSocketAdapter(config WebSocketConfig) *WebSocketAdapter {
	if config.Headers == nil {
		config.Headers = make(map[string]string)
	}
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = 10 * time.Second
	}
	if config.ReconnectDelay == 0 {
		config.ReconnectDelay = 5 * time.Second
	}
	if config.ResponseTimeout == 0 {
		config.ResponseTimeout = 30 * time.Second
	}

	return &WebSocketAdapter{
		config:  config,
		eventCh: make(chan []byte, 100),
		logger:  slog.Default(),
	}
}

// Connect 连接到WebSocket服务器
func (a *WebSocketAdapter) Connect(ctx context.Context) error {
	a.ctx, a.cancel = context.WithCancel(ctx)

	if err := a.connect(); err != nil {
		return fmt.Errorf("initial connection failed: %w", err)
	}

	// 启动消息处理
	go a.handleMessages()

	// 启动重连协程
	if a.config.AutoReconnect {
		go a.reconnectLoop()
	}

	// 启动心跳
	if a.config.PingInterval > 0 {
		go a.heartbeatLoop()
	}

	return nil
}

// connect 建立WebSocket连接
func (a *WebSocketAdapter) connect() error {
	u, err := url.Parse(a.config.URL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	headers := http.Header{}
	if a.config.AccessToken != "" {
		headers.Set("Authorization", "Bearer "+a.config.AccessToken)
	}

	for key, value := range a.config.Headers {
		headers.Set(key, value)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: a.config.ConnectTimeout,
	}

	conn, _, err := dialer.Dial(u.String(), headers)
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	if a.config.ReadTimeout > 0 {
		conn.SetReadDeadline(time.Now().Add(a.config.ReadTimeout))
	}
	if a.config.WriteTimeout > 0 {
		conn.SetWriteDeadline(time.Now().Add(a.config.WriteTimeout))
	}

	// 设置Pong处理器来刷新读取超时
	conn.SetPongHandler(func(appData string) error {
		a.logger.Debug("Received pong")
		if a.config.ReadTimeout > 0 {
			conn.SetReadDeadline(time.Now().Add(a.config.ReadTimeout))
		}
		return nil
	})

	a.mu.Lock()
	a.conn = conn
	a.mu.Unlock()

	a.connected.Store(true)
	a.logger.Info("WebSocket connected", "url", a.config.URL)
	return nil
}

// handleMessages 处理WebSocket消息
func (a *WebSocketAdapter) handleMessages() {
	defer func() {
		if r := recover(); r != nil {
			a.logger.Error("WebSocket handler panic recovered", "error", r)
		}
	}()

	for {
		select {
		case <-a.ctx.Done():
			return
		default:
			a.mu.RLock()
			conn := a.conn
			a.mu.RUnlock()

			if conn == nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			_, message, err := conn.ReadMessage()
			if err != nil {
				a.logger.Error("WebSocket read error", "error", err)
				a.closeConnection()
				continue
			}

			a.messagesRecv.Add(1)

			// 检查是否是API响应
			var apiResp APIResponse
			if err := json.Unmarshal(message, &apiResp); err == nil && apiResp.Echo != "" {
				// 这是API响应，发送给等待的调用者
				if ch, ok := a.pendingCalls.LoadAndDelete(apiResp.Echo); ok {
					if respCh, ok := ch.(chan *APIResponse); ok {
						select {
						case respCh <- &apiResp:
						case <-time.After(1 * time.Second):
							a.logger.Warn("API response channel timeout", "echo", apiResp.Echo)
						}
					}
				}
				continue
			}

			// 检查是否是心跳事件
			if a.handleHeartbeat(message) {
				continue
			}

			// 否则作为事件分发
			select {
			case a.eventCh <- message:
			case <-a.ctx.Done():
				return
			default:
				a.logger.Warn("Event channel full, dropping message")
			}
		}
	}
}

// reconnectLoop 重连循环
func (a *WebSocketAdapter) reconnectLoop() {
	ticker := time.NewTicker(a.config.ReconnectDelay)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if !a.connected.Load() {
				a.logger.Info("Attempting to reconnect...")
				if err := a.connect(); err != nil {
					a.logger.Error("Reconnection failed", "error", err)
				} else {
					a.reconnectCount.Add(1)
					a.logger.Info("Reconnected successfully", "count", a.reconnectCount.Load())
				}
			}
		}
	}
}

// handleHeartbeat 处理心跳事件
func (a *WebSocketAdapter) handleHeartbeat(message []byte) bool {
	var event HeartbeatMetaEvent
	if err := json.Unmarshal(message, &event); err != nil {
		return false
	}

	// 检查是否是心跳事件
	if event.PostType != PostTypeMetaEvent || event.MetaEventType != MetaEventTypeHeartbeat {
		return false
	}

	a.logger.Debug("Received heartbeat",
		"interval", event.Interval,
		"status", event.Status)

	// 更新最后心跳时间
	a.heartbeatMu.Lock()
	a.lastHeartbeat = time.Now()

	// 重置心跳检测计时器
	if a.heartbeatTimer != nil {
		a.heartbeatTimer.Stop()
	}

	// 设置新的超时检测，允许一定的容错时间
	heartbeatTimeout := time.Duration(event.Interval)*time.Millisecond + 10*time.Second
	a.heartbeatTimer = time.AfterFunc(heartbeatTimeout, func() {
		a.logger.Warn("Heartbeat timeout, connection may be lost",
			"last_heartbeat", a.lastHeartbeat,
			"timeout", heartbeatTimeout)
		a.closeConnection()
	})
	a.heartbeatMu.Unlock()

	// 刷新WebSocket读取超时
	if a.config.ReadTimeout > 0 {
		a.mu.RLock()
		if a.conn != nil {
			a.conn.SetReadDeadline(time.Now().Add(a.config.ReadTimeout))
		}
		a.mu.RUnlock()
	}

	return true
}

// heartbeatLoop 心跳循环
func (a *WebSocketAdapter) heartbeatLoop() {
	ticker := time.NewTicker(a.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.mu.RLock()
			conn := a.conn
			a.mu.RUnlock()

			if conn != nil {
				if a.config.WriteTimeout > 0 {
					conn.SetWriteDeadline(time.Now().Add(a.config.WriteTimeout))
				}
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					a.logger.Warn("Ping failed", "error", err)
					a.closeConnection()
				}
			}
		}
	}
}

// CallAction 调用API
func (a *WebSocketAdapter) CallAction(actionName string, params map[string]any) ([]byte, error) {
	if !a.connected.Load() {
		return nil, fmt.Errorf("websocket not connected")
	}

	// 生成请求ID
	id := a.callID.Add(1)
	echo := fmt.Sprintf("call_%d_%d", time.Now().UnixNano(), id)

	// 创建响应通道
	respCh := make(chan *APIResponse, 1)
	a.pendingCalls.Store(echo, respCh)

	// 清理函数
	defer func() {
		a.pendingCalls.Delete(echo)
		close(respCh)
	}()

	// 构造请求
	request := map[string]any{
		"action": actionName,
		"params": params,
		"echo":   echo,
	}

	// 发送请求
	if err := a.sendJSON(request); err != nil {
		return nil, fmt.Errorf("send request failed: %w", err)
	}

	// 等待响应
	ctx, cancel := context.WithTimeout(a.ctx, a.config.ResponseTimeout)
	defer cancel()

	select {
	case resp := <-respCh:
		// 将响应重新序列化为标准格式返回
		return json.Marshal(resp)
	case <-ctx.Done():
		return nil, fmt.Errorf("API call timeout: %s", actionName)
	}
}

// sendJSON 发送JSON消息
func (a *WebSocketAdapter) sendJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal JSON failed: %w", err)
	}

	a.mu.RLock()
	conn := a.conn
	a.mu.RUnlock()

	if conn == nil || !a.connected.Load() {
		return fmt.Errorf("websocket not connected")
	}

	if a.config.WriteTimeout > 0 {
		conn.SetWriteDeadline(time.Now().Add(a.config.WriteTimeout))
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		a.closeConnection()
		return fmt.Errorf("write message failed: %w", err)
	}

	a.messagesSent.Add(1)
	return nil
}

// EventChannel 获取事件通道
func (a *WebSocketAdapter) EventChannel() <-chan []byte {
	return a.eventCh
}

// ParseEvent 解析事件
func (a *WebSocketAdapter) ParseEvent(rawEvent []byte) (Event, error) {
	return UnmarshalEvent(rawEvent)
}

// Disconnect 断开连接
func (a *WebSocketAdapter) Disconnect() error {
	if a.cancel != nil {
		a.cancel()
	}

	a.closeConnection()
	close(a.eventCh)

	a.logger.Info("WebSocket adapter disconnected")
	return nil
}

// closeConnection 关闭连接
func (a *WebSocketAdapter) closeConnection() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.conn != nil {
		a.conn.Close()
		a.conn = nil
	}
	a.connected.Store(false)

	// 清理心跳计时器
	a.heartbeatMu.Lock()
	if a.heartbeatTimer != nil {
		a.heartbeatTimer.Stop()
		a.heartbeatTimer = nil
	}
	a.heartbeatMu.Unlock()
}

// IsConnected 检查连接状态
func (a *WebSocketAdapter) IsConnected() bool {
	return a.connected.Load()
}

// Stats 获取统计信息
func (a *WebSocketAdapter) Stats() interface{} {
	a.heartbeatMu.RLock()
	lastHeartbeat := a.lastHeartbeat
	a.heartbeatMu.RUnlock()

	return map[string]interface{}{
		"messages_sent":     a.messagesSent.Load(),
		"messages_received": a.messagesRecv.Load(),
		"reconnect_count":   a.reconnectCount.Load(),
		"connected":         a.connected.Load(),
		"last_heartbeat":    lastHeartbeat,
		"heartbeat_active":  !lastHeartbeat.IsZero(),
	}
}
