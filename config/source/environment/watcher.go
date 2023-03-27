package environment

import (
	"context"
	"time"

	"github.com/moderntv/cadre/config/source"
)

var _ source.Watcher = &watcher{}

type watcher struct {
	prefix string
	check  func(c chan source.ConfigChange)
}

func newWatcher(prefix string, check func(c chan source.ConfigChange)) (w *watcher) {
	w = &watcher{
		prefix: prefix,
		check:  check,
	}
	return
}

func (w *watcher) C(ctx context.Context) chan source.ConfigChange {
	c := make(chan source.ConfigChange)

	go func() {
		for {
			select {
			case <-ctx.Done():
				close(c)
				return

			case <-time.After(5 * time.Second):
				w.check(c)
			}
		}
	}()

	return c
}
