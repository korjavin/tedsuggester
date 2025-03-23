package scheduler

import (
	"context"
	"log"
	"time"
)

type Scheduler struct {
	interval time.Duration
	tasks    []Task
	stopChan chan struct{}
}

type Task struct {
	Name     string
	Schedule time.Weekday
	Time     time.Time
	Handler  func(ctx context.Context) error
}

func New() *Scheduler {
	return &Scheduler{
		interval: 1 * time.Minute, // Check every minute
		stopChan: make(chan struct{}),
	}
}

func (s *Scheduler) AddTask(task Task) {
	s.tasks = append(s.tasks, task)
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runScheduledTasks(ctx)
		case <-s.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scheduler) Stop() {
	close(s.stopChan)
}

func (s *Scheduler) runScheduledTasks(ctx context.Context) {
	now := time.Now()
	currentWeekday := now.Weekday()
	currentTime := time.Date(0, 0, 0, now.Hour(), now.Minute(), 0, 0, time.UTC)

	for _, task := range s.tasks {
		if task.Schedule == currentWeekday && task.Time == currentTime {
			go func(t Task) {
				if err := t.Handler(ctx); err != nil {
					log.Printf("Task %s failed: %v", t.Name, err)
				}
			}(task)
		}
	}
}

// WeeklySchedule creates a weekly schedule for a task
func WeeklySchedule(weekday time.Weekday, hour, minute int) time.Time {
	return time.Date(0, 0, 0, hour, minute, 0, 0, time.UTC)
}
