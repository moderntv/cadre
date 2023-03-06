package config

import (
	"context"
	"sync"

	"github.com/moderntv/cadre/config/source"
)

type watcher struct {
	ctx       context.Context
	ctxCancel func()
	watchers  []source.Watcher
}

func newWatcher(srcs ...source.Source) (w *watcher, err error) {
	watchers := make([]source.Watcher, len(srcs))
	for i, src := range srcs {
		watchers[i], err = src.Watch()
		if err != nil {
			return
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	w = &watcher{
		ctx:       ctx,
		ctxCancel: cancel,
		watchers:  watchers,
	}

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
