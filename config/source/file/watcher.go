package file

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/moderntv/cadre/config/source"
)

var _ source.Watcher = &watcher{}

type watcher struct {
	path string
	fsnw *fsnotify.Watcher
}

func newWatcher(path string) (w *watcher, err error) {
	fsnw, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}

	err = fsnw.Add(path)
	if err != nil {
		return
	}

	w = &watcher{
		path: path,
		fsnw: fsnw,
	}
	return
}

func (w *watcher) C(ctx context.Context) chan source.ConfigChange {
	// TODO: create new fsnotify watcher for every channel
	c := make(chan source.ConfigChange)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case event := <-w.fsnw.Events:
				cc := source.ConfigChange{
					SourceName: Name,
				}

				switch event.Op {
				case fsnotify.Remove, fsnotify.Create: //, fsnotify.Chmod:
					cc.Type = "remove/create"
					break
				case fsnotify.Rename:
					_, err := os.Stat(event.Name)
					if err == nil || errors.Is(err, fs.ErrExist) {
						_ = w.fsnw.Add(event.Name)
					}

					cc.Type = "rename"
					break
				case fsnotify.Chmod, fsnotify.Write:
					cc.Type = "write"
					w.fsnw.Remove(w.path)
				}

				c <- cc
				w.fsnw.Add(w.path)
			}
		}
	}()

	return c
}

func (w *watcher) Stop() {
	w.fsnw.Close()
}
