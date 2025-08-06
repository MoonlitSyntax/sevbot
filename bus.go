package sevbot

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// EventBus 简化的事件总线，移除Context依赖
type EventBus struct {
	handlers    map[reflect.Type][]EventHandlerFunc
	middlewares []Middleware
	mu          sync.RWMutex
}

// EventHandlerFunc 支持上下文的事件处理函数
type EventHandlerFunc func(ctx context.Context, event Event) error

// Middleware 中间件函数
type Middleware func(next EventHandlerFunc) EventHandlerFunc

func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[reflect.Type][]EventHandlerFunc),
	}
}

func (b *EventBus) Use(middleware Middleware) {
	b.middlewares = append(b.middlewares, middleware)
}

func (b *EventBus) Subscribe(eventType reflect.Type, handler EventHandlerFunc) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[eventType] = append(b.handlers[eventType], handler)
	return nil
}

// dispatch 分发事件，移除Context复杂性
func (b *EventBus) dispatch(client *Client, rawEvent []byte) error {
	// 解析事件
	event, err := client.adapter.ParseEvent(rawEvent)
	if err != nil {
		return fmt.Errorf("failed to parse event: %w", err)
	}

	eventType := reflect.TypeOf(event)

	// 获取处理器
	b.mu.RLock()
	handlers := make([]EventHandlerFunc, len(b.handlers[eventType]))
	copy(handlers, b.handlers[eventType])
	b.mu.RUnlock()

	if len(handlers) == 0 {
		return nil
	}

	// 生成请求ID用于日志
	requestID := generateRequestID()
	logger := client.logger.With(
		"request_id", requestID,
		"event_type", eventType.String(),
	)

	var wg sync.WaitGroup
	errorChan := make(chan error, len(handlers))
	startTime := time.Now()

	// 并发执行所有处理器
	for _, handler := range handlers {
		// 获取信号量
		select {
		case client.semaphore <- struct{}{}:
		case <-client.ctx.Done():
			return fmt.Errorf("context cancelled, cannot dispatch event")
		}

		wg.Add(1)
		client.handlerWG.Add(1) // 添加到客户端的 WaitGroup
		go func(h EventHandlerFunc) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Event handler panic",
						"error", r,
						"event_type", eventType.String())
					errorChan <- fmt.Errorf("panic recovered: %v", r)
				}
				<-client.semaphore
				client.handlerWG.Done() // 完成时通知客户端 WaitGroup
				wg.Done()
			}()

			// 应用中间件
			finalHandler := h
			for i := len(b.middlewares) - 1; i >= 0; i-- {
				finalHandler = b.middlewares[i](finalHandler)
			}

			// 创建带超时的上下文
			ctx, cancel := context.WithTimeout(client.ctx, 30*time.Second)
			defer cancel()

			// 执行处理器
			if err := finalHandler(ctx, event); err != nil {
				logger.Error("Event handler error",
					"error_type", getErrorType(err),
					"error_code", getErrorCode(err),
					"error", err.Error())

				// 使用错误处理器
				if handledErr := client.errorHandler.HandleError(requestID, event, err); handledErr != nil {
					errorChan <- handledErr
				}
			}
		}(handler)
	}

	// 等待所有处理器完成
	go func() {
		wg.Wait()
		close(errorChan)
	}()

	// 收集错误
	var errors []error
	for err := range errorChan {
		if err != nil {
			errors = append(errors, err)
		}
	}

	// 记录执行完成
	duration := time.Since(startTime)
	if len(errors) > 0 {
		logger.Info("Handler executed",
			"duration", duration,
			"error", fmt.Sprintf("multiple errors: %v", errors))
	} else {
		logger.Info("Handler executed",
			"duration", duration,
			"error", nil)
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors occurred during event handling: %v", errors)
	}

	return nil
}

// 辅助函数
func getErrorType(err error) string {
	if botErr, ok := err.(*BotError); ok {
		return string(botErr.Type)
	}
	return "unknown"
}

func getErrorCode(err error) int {
	if botErr, ok := err.(*BotError); ok {
		return botErr.Code
	}
	return 0
}

func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}
