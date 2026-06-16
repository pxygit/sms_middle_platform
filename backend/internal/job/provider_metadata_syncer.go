package job

import (
	"context"
	"log"
	"time"

	"sms-middle-platform/backend/internal/service"
)

type ProviderMetadataSyncer struct {
	meta *service.ProviderMetadataService
	hour int
}

func NewProviderMetadataSyncer(meta *service.ProviderMetadataService) *ProviderMetadataSyncer {
	return &ProviderMetadataSyncer{meta: meta, hour: 6}
}

func (s *ProviderMetadataSyncer) Start(ctx context.Context) {
	go func() {
		for {
			timer := time.NewTimer(time.Until(nextDailyTime(s.hour)))
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				if err := s.meta.SyncAll(ctx); err != nil {
					log.Printf("sync provider metadata: %v", err)
				}
			}
		}
	}()
}

func nextDailyTime(hour int) time.Time {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}
