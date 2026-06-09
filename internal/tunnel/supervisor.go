package tunnel

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/yuanboshe/llm-relay/internal/config"
)

// Starter launches a tunnel process.
type Starter interface {
	Start(ctx context.Context, cfg config.Config, stderr io.Writer) (*Process, error)
}

// StarterFunc adapts a function into a Starter.
type StarterFunc func(ctx context.Context, cfg config.Config, stderr io.Writer) (*Process, error)

// Start launches a tunnel process.
func (f StarterFunc) Start(ctx context.Context, cfg config.Config, stderr io.Writer) (*Process, error) {
	return f(ctx, cfg, stderr)
}

// State describes tunnel supervision state.
type State string

const (
	StateStopped  State = "stopped"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateRetrying State = "retrying"
	StateDisabled State = "disabled"
)

// Status reports tunnel supervision state.
type Status struct {
	State         State
	Attempts      int
	LastError     string
	LastStartedAt time.Time
	LastExitAt    time.Time
	Backoff       time.Duration
}

// Supervisor restarts the SSH tunnel when it exits unexpectedly.
type Supervisor struct {
	Starter    Starter
	Pause      func(context.Context, time.Duration) error
	MinBackoff time.Duration
	MaxBackoff time.Duration

	mu     sync.Mutex
	status Status
}

// Status returns the latest supervision status.
func (s *Supervisor) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Run keeps the tunnel alive until ctx is canceled.
func (s *Supervisor) Run(ctx context.Context, cfg config.Config, stderr io.Writer) error {
	starter := s.Starter
	if starter == nil {
		starter = StarterFunc(Start)
	}
	pause := s.Pause
	if pause == nil {
		pause = func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		}
	}
	minBackoff := s.MinBackoff
	if minBackoff <= 0 {
		minBackoff = time.Second
	}
	maxBackoff := s.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = 30 * time.Second
	}
	backoff := minBackoff
	attempts := 0

	for {
		if err := ctx.Err(); err != nil {
			s.setStatus(Status{State: StateStopped, Attempts: attempts})
			return err
		}

		attempts++
		s.setStatus(Status{
			State:    StateStarting,
			Attempts: attempts,
			Backoff:  backoff,
		})
		if stderr != nil {
			_, _ = fmt.Fprintf(stderr, "tunnel starting (attempt %d)\n", attempts)
		}

		process, err := starter.Start(ctx, cfg, stderr)
		if err != nil {
			s.setStatus(Status{
				State:      StateRetrying,
				Attempts:   attempts,
				LastError:  err.Error(),
				Backoff:    backoff,
				LastExitAt: time.Now(),
			})
			if stderr != nil {
				_, _ = fmt.Fprintf(stderr, "tunnel start failed: %v; retrying in %s\n", err, backoff)
			}
			if err := pause(ctx, backoff); err != nil {
				s.setStatus(Status{State: StateStopped, Attempts: attempts, LastError: err.Error()})
				return err
			}
			backoff = nextBackoff(backoff, maxBackoff)
			continue
		}

		s.setStatus(Status{
			State:         StateRunning,
			Attempts:      attempts,
			LastStartedAt: time.Now(),
			Backoff:       backoff,
		})
		if stderr != nil {
			_, _ = fmt.Fprintln(stderr, "tunnel connected")
		}

		select {
		case err := <-process.Done():
			if ctx.Err() != nil {
				s.setStatus(Status{State: StateStopped, Attempts: attempts})
				return ctx.Err()
			}
			state := StateRetrying
			lastErr := ""
			if err != nil {
				lastErr = err.Error()
			}
			s.setStatus(Status{
				State:         state,
				Attempts:      attempts,
				LastError:     lastErr,
				LastExitAt:    time.Now(),
				LastStartedAt: time.Now(),
				Backoff:       backoff,
			})
			if stderr != nil {
				if err != nil {
					_, _ = fmt.Fprintf(stderr, "tunnel exited: %v; retrying in %s\n", err, backoff)
				} else {
					_, _ = fmt.Fprintf(stderr, "tunnel exited; retrying in %s\n", backoff)
				}
			}
			if err := pause(ctx, backoff); err != nil {
				s.setStatus(Status{State: StateStopped, Attempts: attempts, LastError: err.Error()})
				return err
			}
			backoff = nextBackoff(backoff, maxBackoff)
		case <-ctx.Done():
			s.setStatus(Status{State: StateStopped, Attempts: attempts})
			return ctx.Err()
		}
	}
}

func (s *Supervisor) setStatus(status Status) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next <= 0 {
		next = time.Second
	}
	if next > max {
		return max
	}
	return next
}
