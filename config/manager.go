package config

import (
	"fmt"

	"github.com/moderntv/cadre/config/source"
)

type Manager struct {
	sources []source.Source

	watcher        *watcher
	watchPublishCh chan source.ConfigChange
	watchSubCh     chan chan source.ConfigChange
	watchUnsubCh   chan chan source.ConfigChange
}

func NewManager(opts ...Option) (m *Manager, err error) {
	options := defaultOptions()
	for _, opt := range opts {
		err = opt(options)
		if err != nil {
			return
		}
	}

	m = &Manager{
		sources: options.sources,

		watchPublishCh: make(chan source.ConfigChange, 1),
		watchSubCh:     make(chan chan source.ConfigChange, 1),
		watchUnsubCh:   make(chan chan source.ConfigChange, 1),
	}

	go m.manageSubscribers()

	return
}

func (m *Manager) Load(dst any) (err error) {
	for _, src := range m.sources {
		err = src.Load(dst)
		if err != nil {
			err = fmt.Errorf("source `%s` failed to load: %w", src.Name(), err)
			return
		}
	}

	return
}

// Subscribe returns a channel which will receive message on change
func (m *Manager) Subscribe() (chan source.ConfigChange, error) {
	if m.watcher == nil {
		var err error
		m.watcher, err = newWatcher(m.sources...)
		if err != nil {
			return nil, err
		}
	}

	msgCh := m.watcher.C()
	m.watchSubCh <- msgCh
	return msgCh, nil
}

// Subscribe returns a channel which will receive message on change
func (m *Manager) Unsubscribe(msgCh chan source.ConfigChange) {
	m.watchUnsubCh <- msgCh
}

func (m *Manager) manageSubscribers() {
	subs := map[chan source.ConfigChange]struct{}{}
	for {
		select {
		// case <-c.stopCh:
		//     return
		case msgCh := <-m.watchSubCh:
			subs[msgCh] = struct{}{}
		case msgCh := <-m.watchUnsubCh:
			delete(subs, msgCh)
		case msg := <-m.watchPublishCh:
			for msgCh := range subs {
				// msgCh is buffered, use non-blocking send to protect the broker:
				select {
				case msgCh <- msg:
				default:
				}
			}
		}
	}
}
