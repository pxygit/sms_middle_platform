package job

import (
	"context"
	"time"

	"sms-middle-platform/backend/internal/service"
)

type Poller struct {
	orders   *service.OrderService
	interval time.Duration
}

func NewPoller(orders *service.OrderService, interval time.Duration) *Poller {
	return &Poller{orders: orders, interval: interval}
}

func (p *Poller) Start(ctx context.Context) {
	if p.interval <= 0 {
		p.interval = 8 * time.Second
	}
	ticker := time.NewTicker(p.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = p.orders.PollActive(ctx, time.Now().Add(-p.interval))
			}
		}
	}()
}
