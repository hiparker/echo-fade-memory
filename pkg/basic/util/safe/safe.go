// Package safe provides panic-safe goroutine utilities.
// Go: fire-and-forget goroutine with recovery.
// Run: synchronous execution with recovery.
// Group: errgroup with built-in panic recovery.
package safe

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"

	"golang.org/x/sync/errgroup"
)

// Go starts f in a new goroutine with panic recovery. Panics do not crash the process.
func Go(f func(), onPanic func(error)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				err := fmt.Errorf("panic recovered: %v\n%s", r, debug.Stack())
				if onPanic != nil {
					onPanic(err)
				} else {
					log.Printf("panic recovered: %v", err)
				}
			}
		}()
		f()
	}()
}

// Run runs f synchronously with panic recovery. Returns error if panic occurred.
func Run(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic recovered: %v\n%s", r, debug.Stack())
		}
	}()
	return f()
}

// Wrap returns a function that runs f with panic recovery.
func Wrap(f func() error) func() error {
	return func() error { return Run(f) }
}

// Group is like errgroup.Group but Go() has panic recovery built-in.
type Group struct {
	g *errgroup.Group
}

// WithContext returns a Group and derived context.
func WithContext(ctx context.Context) (*Group, context.Context) {
	g, ctx := errgroup.WithContext(ctx)
	return &Group{g: g}, ctx
}

// Go runs f in a goroutine. Panics are recovered and returned as error from Wait().
func (g *Group) Go(f func() error) {
	g.g.Go(func() error { return Run(f) })
}

// Wait blocks until all goroutines finish.
func (g *Group) Wait() error {
	return g.g.Wait()
}
