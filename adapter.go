package sevbot

import "context"

// Adapter 适配器接口
type Adapter interface {
	// Connect 连接到服务器
	Connect(ctx context.Context) error

	// Disconnect 断开连接
	Disconnect() error

	// CallAction 调用 API
	CallAction(ctx context.Context, actionName string, params map[string]any) ([]byte, error)

	// EventChannel 获取事件通道
	EventChannel() <-chan []byte

	// ParseEvent 解析事件
	ParseEvent(rawEvent []byte) (Event, error)
}

// ConnectableAdapter 支持连接状态检查的适配器
type ConnectableAdapter interface {
	Adapter

	// IsConnected 检查是否已连接
	IsConnected() bool
}

// StatsAdapter 支持统计信息的适配器
type StatsAdapter interface {
	Adapter

	// Stats 获取统计信息
	Stats() interface{}
}
