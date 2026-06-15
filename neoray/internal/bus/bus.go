package bus

import (
	"context"
	"sync"

	"neoray/internal/logger"
)

// MessageHandler 消息处理函数类型
type MessageHandler func(ctx context.Context, msg *InboundMessage) error

// MessageBus 消息总线
type MessageBus struct {
	// 入站消息队列
	inboundQueue chan *InboundMessage
	// 出站消息队列
	outboundQueue chan *OutboundMessage

	// 入站消息处理器
	inboundHandlers []MessageHandler
	// 出站消息订阅者
	outboundSubscribers map[string]chan<- *OutboundMessage

	// 状态
	mu         sync.RWMutex
	running    bool
	stopChan   chan struct{}
	wg         sync.WaitGroup

	// 队列大小监控
	inboundSize  int
	outboundSize int
}

// NewMessageBus 创建消息总线
func NewMessageBus(inboundSize, outboundSize int) *MessageBus {
	if inboundSize <= 0 {
		inboundSize = 100
	}
	if outboundSize <= 0 {
		outboundSize = 100
	}
	return &MessageBus{
		inboundQueue:        make(chan *InboundMessage, inboundSize),
		outboundQueue:       make(chan *OutboundMessage, outboundSize),
		inboundHandlers:     make([]MessageHandler, 0),
		outboundSubscribers: make(map[string]chan<- *OutboundMessage),
		stopChan:            make(chan struct{}),
	}
}

// Start 启动消息总线
func (b *MessageBus) Start() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return nil
	}

	b.running = true
	b.stopChan = make(chan struct{})

	// 启动入站处理协程
	b.wg.Add(1)
	go b.processInbound()

	// 启动出站分发协程
	b.wg.Add(1)
	go b.processOutbound()

	return nil
}

// Stop 停止消息总线
func (b *MessageBus) Stop() error {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return nil
	}
	b.running = false
	close(b.stopChan)
	b.mu.Unlock()

	// 等待所有协程退出
	b.wg.Wait()

	// 关闭队列
	close(b.inboundQueue)
	close(b.outboundQueue)

	return nil
}

// PublishInbound 发布入站消息
func (b *MessageBus) PublishInbound(msg *InboundMessage) error {
	b.mu.RLock()
	if !b.running {
		b.mu.RUnlock()
		return ErrBusNotRunning
	}
	b.mu.RUnlock()

	select {
	case b.inboundQueue <- msg:
		b.mu.Lock()
		b.inboundSize++
		b.mu.Unlock()
		return nil
	case <-b.stopChan:
		return ErrBusStopped
	}
}

// PublishOutbound 发布出站消息
func (b *MessageBus) PublishOutbound(msg *OutboundMessage) error {
	b.mu.RLock()
	if !b.running {
		b.mu.RUnlock()
		return ErrBusNotRunning
	}
	b.mu.RUnlock()

	select {
	case b.outboundQueue <- msg:
		b.mu.Lock()
		b.outboundSize++
		b.mu.Unlock()
		return nil
	case <-b.stopChan:
		return ErrBusStopped
	}
}

// RegisterInboundHandler 注册入站消息处理器
func (b *MessageBus) RegisterInboundHandler(handler MessageHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.inboundHandlers = append(b.inboundHandlers, handler)
}

// SubscribeOutbound 订阅出站消息
func (b *MessageBus) SubscribeOutbound(subscriberID string, ch chan<- *OutboundMessage) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.outboundSubscribers[subscriberID]; exists {
		return ErrSubscriberExists
	}
	b.outboundSubscribers[subscriberID] = ch
	return nil
}

// UnsubscribeOutbound 取消订阅
func (b *MessageBus) UnsubscribeOutbound(subscriberID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.outboundSubscribers, subscriberID)
}

// GetQueueSizes 获取队列大小
func (b *MessageBus) GetQueueSizes() (int, int) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.inboundSize, b.outboundSize
}

// IsRunning 检查是否运行中
func (b *MessageBus) IsRunning() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.running
}

// processInbound 处理入站消息
func (b *MessageBus) processInbound() {
	defer b.wg.Done()

	for {
		select {
		case msg := <-b.inboundQueue:
			b.mu.Lock()
			b.inboundSize--
			handlers := make([]MessageHandler, len(b.inboundHandlers))
			copy(handlers, b.inboundHandlers)
			b.mu.Unlock()

			// 调用所有处理器
			for _, handler := range handlers {
				if err := handler(msg.Context, msg); err != nil {
					logger.Error("Inbound handler error",
						logger.String("msg_id", msg.ID),
						logger.ErrorField(err))
				}
			}

		case <-b.stopChan:
			return
		}
	}
}

// processOutbound 处理出站消息
func (b *MessageBus) processOutbound() {
	defer b.wg.Done()

	for {
		select {
		case msg := <-b.outboundQueue:
			b.mu.Lock()
			b.outboundSize--
			subscribers := make(map[string]chan<- *OutboundMessage, len(b.outboundSubscribers))
			for id, ch := range b.outboundSubscribers {
				subscribers[id] = ch
			}
			b.mu.Unlock()

			// 分发到所有订阅者
			for id, ch := range subscribers {
				select {
				case ch <- msg:
					// 发送成功
				case <-b.stopChan:
					return
				default:
					// 订阅者队列满了，记录日志
					logger.Warn("Outbound subscriber queue full, dropping message",
						logger.String("subscriber", id),
						logger.String("msg_id", msg.ID))
				}
			}

		case <-b.stopChan:
			return
		}
	}
}
