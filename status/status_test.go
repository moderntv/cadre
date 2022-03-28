package status

import (
	"os"
	"reflect"
	"testing"
)

var (
	hostname string
	err      error
)

func init() {
	hostname, err = os.Hostname()
	if err != nil {
		panic(err)
	}
}

func TestNewStatus(t *testing.T) {
	type args struct {
		version string
	}
	tests := []struct {
		name string
		args args
		want *Status
	}{
		{
			name: "simple",
			args: args{
				version: "v6.6.6",
			},
			want: &Status{
				version:    "v6.6.6",
				hostname:   hostname,
				components: map[string]*ComponentStatus{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewStatus(tt.args.version); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatus_Register(t *testing.T) {
	tests := []struct {
		name    string
		status  *Status
		service string
	}{
		{
			name:    "simple",
			status:  NewStatus("v6.6.6"),
			service: "foobar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.status.Register(tt.service)
			if err != nil {
				t.Errorf("component status Register() returned error: %v", err)
			}

			_, ok := tt.status.components[tt.service]
			if !ok {
				t.Errorf("Register() has not registered service `%s`. components = %v", tt.service, tt.status.components)
			}
		})
	}
}

func TestStatus_Report(t *testing.T) {
	type serviceStatus struct {
		status  StatusType
		message string
	}
	tests := []struct {
		name               string
		status             *Status
		services           []string
		servicesChanges    []map[string]serviceStatus
		servicesFinal      map[string]StatusType
		overallStatusFinal StatusType
	}{
		{
			name:     "simple-ok",
			status:   NewStatus("v6.6.6"),
			services: []string{"foo", "bar"},
			servicesChanges: []map[string]serviceStatus{
				{
					"foo": {OK, "OK"},
					"bar": {OK, "OK"},
				},
			},
			servicesFinal: map[string]StatusType{
				"foo": OK,
				"bar": OK,
			},
			overallStatusFinal: OK,
		},
		{
			name:     "all-errors",
			status:   NewStatus("v6.6.6"),
			services: []string{"foo", "bar"},
			servicesChanges: []map[string]serviceStatus{
				{
					"foo": {ERROR, "Major failure"},
					"bar": {ERROR, "Major failure"},
				},
			},
			servicesFinal: map[string]StatusType{
				"foo": ERROR,
				"bar": ERROR,
			},
			overallStatusFinal: ERROR,
		},
		{
			name:            "uninitialized",
			status:          NewStatus("v6.6.6"),
			services:        []string{"foo", "bar"},
			servicesChanges: []map[string]serviceStatus{},
			servicesFinal: map[string]StatusType{
				"foo": ERROR,
				"bar": ERROR,
			},
			overallStatusFinal: ERROR,
		},
		{
			name:     "warning",
			status:   NewStatus("v6.6.6"),
			services: []string{"foo", "bar"},
			servicesChanges: []map[string]serviceStatus{
				{
					"foo": {OK, "OK"},
					"bar": {WARN, "Minor failure"},
				},
			},
			servicesFinal: map[string]StatusType{
				"foo": OK,
				"bar": WARN,
			},
			overallStatusFinal: WARN,
		},
		{
			name:     "warning-error",
			status:   NewStatus("v6.6.6"),
			services: []string{"k", "foo", "bar", "meh"},
			servicesChanges: []map[string]serviceStatus{
				{
					"k":   {OK, "OK"},
					"meh": {WARN, "Meh failure"},
					"foo": {ERROR, "Major failure"},
					"bar": {WARN, "Meh failure"},
				},
				{
					"k":   {OK, "OK"},
					"meh": {WARN, "Meh failure"},
					"foo": {WARN, "Meh failure"},
					"bar": {ERROR, "Major failure"},
				},
				{
					"k":   {OK, "OK"},
					"meh": {WARN, "Meh failure"},
					"foo": {ERROR, "Major failure"},
					"bar": {ERROR, "Major failure"},
				},
			},
			servicesFinal: map[string]StatusType{
				"k":   OK,
				"meh": WARN,
				"foo": ERROR,
				"bar": ERROR,
			},
			overallStatusFinal: ERROR,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// register services
			componentStatuses := map[string]*ComponentStatus{}
			for _, service := range tt.services {
				var err error
				componentStatuses[service], err = tt.status.Register(service)
				if err != nil {
					t.Errorf("Register() returned error = %v", err)
				}
			}

			// perform the status changes
			for _, changeSet := range tt.servicesChanges {
				for service, change := range changeSet {
					componentStatuses[service].SetStatus(change.status, change.message)
				}
			}

			// get report
			gotReport := tt.status.Report()
			// check per-service status
			for service, finalStatus := range tt.servicesFinal {
				serviceStatus := gotReport.Components[service]

				if serviceStatus.Status != finalStatus {
					t.Errorf("incorrect service status. got = %v, want = %v", serviceStatus.Status, finalStatus)
				}
			}
			// check overall status
			if gotReport.Status != tt.overallStatusFinal {
				t.Errorf("incorrect overall status. got = %v, want = %v", gotReport.Status, tt.overallStatusFinal)
			}
		})
	}
}
