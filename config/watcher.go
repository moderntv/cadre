package config

import (
	"context"
	"fmt"
	"sync"

	"github.com/moderntv/cadre/config/source"
	"github.com/sveatlo/bundlerr"
)

type watcher struct {
	ctx       context.Context
	ctxCancel func()
	watchers  []source.Watcher
}

func newWatcher(ctx context.Context, srcs ...source.Source) (w *watcher, err error) {
	watchers := make([]source.Watcher, 0, len(srcs))
	b := bundlerr.New()
	for _, src := range srcs {
		watcher, err := src.Watch()
		if err != nil {
			b.Append(fmt.Errorf("source %s watcher ended with err: %w", src.Name(), err))
			continue
		}

		watchers = append(watchers, watcher)
	}

	watcherContext, cancel := context.WithCancel(ctx)
	w = &watcher{
		ctx:       watcherContext,
		ctxCancel: cancel,
		watchers:  watchers,
	}
	err = b.Evaluate()
	return
}

func (w *watcher) C() chan source.ConfigChange {
	cs := make([]chan source.ConfigChange, len(w.watchers))
	for i, watcher := range w.watchers {
		cs[i] = watcher.C(w.ctx)
	}

	return merge(cs...)
}

func (w *watcher) Stop() {
	w.ctxCancel()
}

func merge(cs ...chan source.ConfigChange) chan source.ConfigChange {
	out := make(chan source.ConfigChange)

	var wg sync.WaitGroup
	wg.Add(len(cs))

	for _, c := range cs {
		go func(c chan source.ConfigChange) {
			for v := range c {
				out <- v
			}
			wg.Done()
		}(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}
