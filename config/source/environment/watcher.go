package environment

import (
	"context"
	"time"

	"github.com/moderntv/cadre/config/source"
)

var _ source.Watcher = &watcher{}

type watcher struct {
	prefix string
}

func newWatcher(prefix string) (w *watcher) {
	w = &watcher{
		prefix: prefix,
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
				c <- source.ConfigChange{
					SourceName: Name,
				}
			}
		}

		close(c)
	}()

	return c
}
