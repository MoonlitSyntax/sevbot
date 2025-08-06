package sevbot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
	connected    atomic.Bool
	messagesSent atomic.Int64
	messagesRecv atomic.Int64

	// API调用管理
	pendingCalls sync.Map // map[string]chan *APIResponse
	callID       atomic.Int64

	// 分离的组件
	heartbeat *HeartbeatManager
	reconnect *ReconnectManager
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

	adapter := &WebSocketAdapter{
		config:  config,
		eventCh: make(chan []byte, 1000), // 增加缓冲区大小
		logger:  slog.Default(),
	}

	// 初始化分离的组件
	adapter.heartbeat = NewHeartbeatManager(config, adapter.logger)
	adapter.heartbeat.SetTimeoutHandler(func() {
		adapter.closeConnection()
	})

	if config.AutoReconnect {
		adapter.reconnect = NewReconnectManager(
			DefaultReconnectStrategy(),
			adapter.connect,
			adapter.IsConnected,
			adapter.logger,
		)
	}

	return adapter
}

// Connect 连接到WebSocket服务器
func (a *WebSocketAdapter) Connect(ctx context.Context) error {
	a.ctx, a.cancel = context.WithCancel(ctx)

	if err := a.connect(); err != nil {
		return fmt.Errorf("initial connection failed: %w", err)
	}

	// 设置组件的上下文
	a.heartbeat.SetContext(a.ctx)
	if a.reconnect != nil {
		a.reconnect.SetContext(a.ctx)
	}

	// 启动消息处理
	go a.handleMessages()

	// 启动分离的组件
	if a.reconnect != nil {
		a.reconnect.Start()
	}
	a.heartbeat.Start()

	// 启动定期清理
	a.startPendingCallCleanup()

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

	// 更新心跳管理器的连接
	a.heartbeat.SetConnection(conn)

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
			if a.heartbeat.HandleHeartbeatEvent(message) {
				continue
			}

			// 否则作为事件分发，使用智能背压策略
			a.handleEventBackpressure(message)
		}
	}
}

// handleEventBackpressure 简化的背压策略：阻塞一段时间，超时则丢弃并记录日志
func (a *WebSocketAdapter) handleEventBackpressure(message []byte) {
	// 首先尝试非阻塞发送
	select {
	case a.eventCh <- message:
		return // 发送成功
	default:
		// 通道已满，尝试短时间阻塞
	}

	// 阻塞等待最多100ms
	select {
	case a.eventCh <- message:
		return // 发送成功
	case <-time.After(100 * time.Millisecond):
		// 超时，记录日志并丢弃消息
		a.logger.Warn("Event channel full, dropping message after timeout",
			"channel_len", len(a.eventCh),
			"channel_cap", cap(a.eventCh))
	case <-a.ctx.Done():
		return
	}
}

// CallAction 调用API
func (a *WebSocketAdapter) CallAction(ctx context.Context, actionName string, params map[string]any) ([]byte, error) {
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

	// 清理所有待处理的API调用
	a.cleanupPendingCalls()
	
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

	// 清理心跳管理器
	a.heartbeat.Stop()
}

// cleanupPendingCalls 清理所有待处理的API调用，防止goroutine泄漏
func (a *WebSocketAdapter) cleanupPendingCalls() {
	a.logger.Info("Cleaning up pending API calls")
	
	// 遍历并关闭所有等待中的响应通道
	pendingCount := 0
	a.pendingCalls.Range(func(key, value interface{}) bool {
		if ch, ok := value.(chan *APIResponse); ok {
			// 发送取消错误并关闭通道
			select {
			case ch <- &APIResponse{
				Status:  "failed",
				RetCode: -1,
				Message: "Connection closed",
			}:
			default:
				// 通道可能已满，直接关闭
			}
			close(ch)
			pendingCount++
		}
		a.pendingCalls.Delete(key)
		return true
	})
	
	if pendingCount > 0 {
		a.logger.Warn("Cleaned up pending API calls", "count", pendingCount)
	}
}

// startPendingCallCleanup 启动定期清理超时的API调用
func (a *WebSocketAdapter) startPendingCallCleanup() {
	go func() {
		ticker := time.NewTicker(30 * time.Second) // 每30秒清理一次
		defer ticker.Stop()
		
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				a.cleanupTimeoutCalls()
			}
		}
	}()
}

// cleanupTimeoutCalls 清理超时的API调用
func (a *WebSocketAdapter) cleanupTimeoutCalls() {
	now := time.Now()
	timeoutDuration := a.config.ResponseTimeout
	if timeoutDuration == 0 {
		timeoutDuration = 30 * time.Second
	}
	
	var timedOutCalls []interface{}
	
	a.pendingCalls.Range(func(key, value interface{}) bool {
		if keyStr, ok := key.(string); ok {
			// 从key中提取时间戳（格式：call_timestamp_id）
			parts := strings.Split(keyStr, "_")
			if len(parts) >= 2 {
				if timestamp, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					callTime := time.Unix(0, timestamp)
					if now.Sub(callTime) > timeoutDuration {
						timedOutCalls = append(timedOutCalls, key)
					}
				}
			}
		}
		return true
	})
	
	// 清理超时的调用
	for _, key := range timedOutCalls {
		if value, ok := a.pendingCalls.LoadAndDelete(key); ok {
			if ch, ok := value.(chan *APIResponse); ok {
				select {
				case ch <- &APIResponse{
					Status:  "failed",
					RetCode: -1,
					Message: "Request timeout",
				}:
				default:
				}
				close(ch)
			}
		}
	}
	
	if len(timedOutCalls) > 0 {
		a.logger.Warn("Cleaned up timeout API calls", "count", len(timedOutCalls))
	}
}

// IsConnected 检查连接状态
func (a *WebSocketAdapter) IsConnected() bool {
	return a.connected.Load()
}

// Stats 获取统计信息
func (a *WebSocketAdapter) Stats() interface{} {
	lastHeartbeat := a.heartbeat.GetLastHeartbeat()
	
	stats := map[string]interface{}{
		"messages_sent":     a.messagesSent.Load(),
		"messages_received": a.messagesRecv.Load(),
		"connected":         a.connected.Load(),
		"last_heartbeat":    lastHeartbeat,
		"heartbeat_active":  !lastHeartbeat.IsZero(),
	}

	if a.reconnect != nil {
		stats["reconnect_count"] = a.reconnect.GetReconnectCount()
	} else {
		stats["reconnect_count"] = 0
	}

	return stats
}
