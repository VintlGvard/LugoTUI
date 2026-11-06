package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	lua "github.com/yuin/gopher-lua"
)

type TaskResult struct {
	ID      uint64
	Action  string
	Payload map[string]any
	Err     error
	owner   *taskState
}

type taskState struct {
	next    uint64
	active  map[uint64]context.CancelFunc
	named   map[string]uint64
	results chan TaskResult
	done    chan struct{}
}

func (r *Runtime) StartTask(action string, work func(context.Context) (map[string]any, error)) (uint64, error) {
	if r.L == nil {
		return 0, errors.New("runtime closed")
	}
	if work == nil {
		return 0, protocolError(CodeValidationError, errors.New("nil task"))
	}
	if r.actions == nil || r.actions.RawGetString(action).Type() != lua.LTFunction {
		return 0, protocolError(CodeValidationError, fmt.Errorf("unknown action %s", action))
	}
	if r.tasks == nil {
		r.tasks = &taskState{active: map[uint64]context.CancelFunc{}, named: map[string]uint64{}, results: make(chan TaskResult, 32), done: make(chan struct{})}
	}
	t := r.tasks
	if len(t.active) >= 32 {
		return 0, protocolError(CodeTaskLimitExceeded, errors.New("32 task limit reached"))
	}
	t.next++
	id := t.next
	ctx, cancel := context.WithCancel(context.Background())
	t.active[id] = cancel
	go func() {
		defer cancel()
		result := TaskResult{ID: id, Action: action, owner: t}
		func() {
			defer func() {
				if p := recover(); p != nil {
					result.Err = fmt.Errorf("task panic: %v", p)
				}
			}()
			result.Payload, result.Err = work(ctx)
		}()
		if ctx.Err() != nil {
			return
		}
		select {
		case t.results <- result:
		case <-ctx.Done():
		case <-t.done:
		}
	}()
	return id, nil
}

func (r *Runtime) CancelTask(id uint64) {
	if r.tasks != nil {
		if cancel := r.tasks.active[id]; cancel != nil {
			cancel()
			delete(r.tasks.active, id)
		}
		for name, namedID := range r.tasks.named {
			if namedID == id {
				delete(r.tasks.named, name)
				break
			}
		}
	}
}

func (r *Runtime) StartTaskNamed(name, action string, work func(context.Context) (map[string]any, error)) (uint64, error) {
	if r.tasks != nil && name != "" {
		if prevID, ok := r.tasks.named[name]; ok {
			r.CancelTask(prevID)
		}
	}
	id, err := r.StartTask(action, work)
	if err == nil && r.tasks != nil && name != "" {
		r.tasks.named[name] = id
	}
	return id, err
}

func (r *Runtime) StartTaskDebounced(name string, delay time.Duration, action string, work func(context.Context) (map[string]any, error)) (uint64, error) {
	return r.StartTaskNamed(name, action, func(ctx context.Context) (map[string]any, error) {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
		return work(ctx)
	})
}

func (r *Runtime) CancelTaskNamed(name string) {
	if r.tasks != nil && name != "" {
		if id, ok := r.tasks.named[name]; ok {
			r.CancelTask(id)
		}
	}
}

func (r *Runtime) TaskResults() <-chan TaskResult {
	if r.tasks == nil {
		return nil
	}
	return r.tasks.results
}

func (r *Runtime) HandleTask(result TaskResult) error {
	if r.L == nil {
		return errors.New("runtime closed")
	}
	if r.tasks == nil || result.owner != r.tasks || r.tasks.active[result.ID] == nil {
		return nil
	}
	r.CancelTask(result.ID)
	if result.Err != nil {
		return result.Err
	}
	return r.Action(result.Action, result.Payload)
}
