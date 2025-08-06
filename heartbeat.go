package sevbot

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// HeartbeatManager 心跳管理器
type HeartbeatManager struct {
	conn           *websocket.Conn
	config         WebSocketConfig
	ctx            context.Context
	mu             sync.RWMutex
	logger         *slog.Logger
	lastHeartbeat  time.Time
	heartbeatTimer *time.Timer
	onTimeout      func() // 心跳超时回调
}

// NewHeartbeatManager 创建心跳管理器
func NewHeartbeatManager(config WebSocketConfig, logger *slog.Logger) *HeartbeatManager {
	return &HeartbeatManager{
		config: config,
		logger: logger,
	}
}

// SetConnection 设置连接
func (h *HeartbeatManager) SetConnection(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conn = conn
}

// SetContext 设置上下文
func (h *HeartbeatManager) SetContext(ctx context.Context) {
	h.ctx = ctx
}

// SetTimeoutHandler 设置超时处理器
func (h *HeartbeatManager) SetTimeoutHandler(handler func()) {
	h.onTimeout = handler
}

// Start 启动心跳
func (h *HeartbeatManager) Start() {
	if h.config.PingInterval > 0 {
		go h.pingLoop()
	}
}

// HandleHeartbeatEvent 处理心跳事件
func (h *HeartbeatManager) HandleHeartbeatEvent(message []byte) bool {
	var event HeartbeatMetaEvent
	if err := json.Unmarshal(message, &event); err != nil {
		return false
	}

	// 检查是否是心跳事件
	if event.PostType != PostTypeMetaEvent || event.MetaEventType != MetaEventTypeHeartbeat {
		return false
	}

	h.logger.Debug("Received heartbeat",
		"interval", event.Interval,
		"status", event.Status)

	// 更新最后心跳时间
	h.mu.Lock()
	h.lastHeartbeat = time.Now()

	// 重置心跳检测计时器
	if h.heartbeatTimer != nil {
		h.heartbeatTimer.Stop()
	}

	// 设置新的超时检测，允许一定的容错时间
	heartbeatTimeout := time.Duration(event.Interval)*time.Millisecond + 10*time.Second
	h.heartbeatTimer = time.AfterFunc(heartbeatTimeout, func() {
		h.logger.Warn("Heartbeat timeout, connection may be lost",
			"last_heartbeat", h.lastHeartbeat,
			"timeout", heartbeatTimeout)
		if h.onTimeout != nil {
			h.onTimeout()
		}
	})
	h.mu.Unlock()

	// 刷新WebSocket读取超时
	if h.config.ReadTimeout > 0 {
		h.mu.RLock()
		if h.conn != nil {
			h.conn.SetReadDeadline(time.Now().Add(h.config.ReadTimeout))
		}
		h.mu.RUnlock()
	}

	return true
}

// pingLoop 心跳发送循环
func (h *HeartbeatManager) pingLoop() {
	ticker := time.NewTicker(h.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.mu.RLock()
			conn := h.conn
			h.mu.RUnlock()

			if conn != nil {
				if h.config.WriteTimeout > 0 {
					conn.SetWriteDeadline(time.Now().Add(h.config.WriteTimeout))
				}
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					h.logger.Warn("Ping failed", "error", err)
					if h.onTimeout != nil {
						h.onTimeout()
					}
				}
			}
		}
	}
}

// Stop 停止心跳管理器
func (h *HeartbeatManager) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.heartbeatTimer != nil {
		h.heartbeatTimer.Stop()
		h.heartbeatTimer = nil
	}
}

// GetLastHeartbeat 获取最后心跳时间
func (h *HeartbeatManager) GetLastHeartbeat() time.Time {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastHeartbeat
}