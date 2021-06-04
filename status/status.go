package status

import (
	"errors"
	"sync"
	"time"
)

type Status struct {
	version string

	cmu        sync.RWMutex
	components map[string]*componentStatus
}

type Report struct {
	Version    string                     `json:"version"`
	Status     StatusType                 `json:"status"`
	Components map[string]componentStatus `json:"components"`
}

func NewStatus(version string) (status *Status) {
	status = &Status{
		version:    version,
		components: map[string]*componentStatus{},
	}

	return
}

func (s *Status) Register(name string) (cs ComponentStatus, err error) {
	s.cmu.Lock()
	defer s.cmu.Unlock()
	if _, ok := s.components[name]; ok {
		err = errors.New("component already registered")
		return
	}

	lcs := &componentStatus{
		CStatus:  ERROR,
		CMessage: "uninitialized",
	}
	s.components[name] = lcs
	cs = lcs

	return
}

func (s *Status) Report() (report Report) {
	s.cmu.RLock()
	defer s.cmu.RUnlock()

	report.Version = s.version
	report.Status = OK
	report.Components = map[string]componentStatus{}
	for n, cs := range s.components {
		if cs == nil {
			continue
		}

		report.Components[n] = componentStatus{
			CStatus:    cs.Status(),
			CMessage:   cs.Message(),
			CUpdatedAt: cs.LastUpdate(),
		}
	}
	for _, s := range report.Components {
		status := s.Status()
		if status == ERROR {
			report.Status = ERROR
			break
		}
		if status == WARN {
			report.Status = WARN
		}
	}

	return
}

type ComponentStatus interface {
	SetStatus(statusType StatusType, message string)
	// getters
	Status() StatusType
	Message() string
	LastUpdate() time.Time
}

type componentStatus struct {
	mu         sync.RWMutex
	CStatus    StatusType `json:"status"`
	CMessage   string     `json:"message,omitempty"`
	CUpdatedAt time.Time  `json:"updated_at,omitempty"`
}

func (cs *componentStatus) SetStatus(statusType StatusType, message string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.CMessage = message
	cs.CStatus = statusType
	cs.CUpdatedAt = time.Now()
}
func (cs *componentStatus) Status() StatusType {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.CStatus
}
func (cs *componentStatus) Message() string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.CMessage
}
func (cs *componentStatus) LastUpdate() time.Time {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.CUpdatedAt
}
