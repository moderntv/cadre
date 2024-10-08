package file

import (
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

func (w *watcher) C() chan source.ConfigChange {
	// TODO: create new fsnotify watcher for every channel
	c := make(chan source.ConfigChange)

	go func() {
		for event := range w.fsnw.Events {
			switch event.Op {
			case fsnotify.Remove, fsnotify.Create: // fsnotify.Chmod:
				continue
			case fsnotify.Rename:
				_, err := os.Stat(event.Name)
				if err == nil || errors.Is(err, fs.ErrExist) {
					_ = w.fsnw.Add(event.Name)
				}
				continue
			case fsnotify.Chmod, fsnotify.Write:
			}

			c <- source.ConfigChange{
				SourceName: Name,
			}
			_ = w.fsnw.Add(w.path)
		}

		close(c)
	}()

	return c
}

func (w *watcher) Stop() {
	w.fsnw.Close()
}
