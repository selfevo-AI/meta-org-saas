package monitoringagent

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

type SchedulerConfig struct {
	Enabled       bool
	DailyTime     string
	LookbackHours int
}

type Scheduler struct {
	service     *Service
	config      SchedulerConfig
	mu          sync.Mutex
	lastRunDate string
	running     bool
}

func NewScheduler(service *Service, config SchedulerConfig) *Scheduler {
	if strings.TrimSpace(config.DailyTime) == "" {
		config.DailyTime = "02:00"
	}
	if config.LookbackHours <= 0 {
		config.LookbackHours = 24
	}
	return &Scheduler{service: service, config: config}
}

func (s *Scheduler) RunIfDue(ctx context.Context, now time.Time) (*MonitoringAgentRun, bool, error) {
	if s == nil || s.service == nil || !s.config.Enabled {
		return nil, false, nil
	}
	runDate := now.Format("2006-01-02")
	if !s.dueAt(now) {
		return nil, false, nil
	}

	s.mu.Lock()
	if s.running || s.lastRunDate == runDate {
		s.mu.Unlock()
		return nil, false, nil
	}
	s.running = true
	s.lastRunDate = runDate
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	run, err := s.service.Run(ctx, RunInput{TriggerType: TriggerScheduled, LookbackHours: s.config.LookbackHours})
	if err != nil {
		return run, true, err
	}
	return run, true, nil
}

func (s *Scheduler) Start(ctx context.Context) {
	if s == nil || !s.config.Enabled {
		return
	}
	ticker := time.NewTicker(time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				if _, executed, err := s.RunIfDue(ctx, now); executed && err != nil {
					log.Printf("monitoring agent scheduled run failed: %v", err)
				}
			}
		}
	}()
}

func (s *Scheduler) dueAt(now time.Time) bool {
	hour, minute, err := parseDailyTime(s.config.DailyTime)
	if err != nil {
		hour, minute = 2, 0
	}
	return now.Hour() > hour || (now.Hour() == hour && now.Minute() >= minute)
}

func parseDailyTime(value string) (int, int, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid daily time")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("daily time out of range")
	}
	return hour, minute, nil
}
