package bus

import "github.com/Kovalyovv/chat-app/internal/domain"

type EventBus struct {
	subscribers chan<- *domain.Message
}

func New(subscribers chan<- *domain.Message) *EventBus {
	return &EventBus{subscribers: subscribers}
}

func (b *EventBus) Publish(msg *domain.Message) {
	b.subscribers <- msg
}
