package status

import (
	"reflect"
	"testing"
)

func TestStatusType_String(t *testing.T) {
	tests := []struct {
		name string
		s    StatusType
		want string
	}{
		{
			name: "ok",
			s:    OK,
			want: "OK",
		},
		{
			name: "warn",
			s:    WARN,
			want: "WARN",
		},
		{
			name: "error",
			s:    ERROR,
			want: "ERROR",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.String(); got != tt.want {
				t.Errorf("StatusType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatusType_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		s       StatusType
		want    []byte
		wantErr bool
	}{
		{
			name: "ok",
			s:    OK,
			want: []byte(`"OK"`),
		},
		{
			name: "warn",
			s:    WARN,
			want: []byte(`"WARN"`),
		},
		{
			name: "error",
			s:    ERROR,
			want: []byte(`"ERROR"`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.s.MarshalJSON()
			if (err != nil) != tt.wantErr {
				t.Errorf("StatusType.MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StatusType.MarshalJSON() = %v, want %v", string(got), string(tt.want))
			}
		})
	}
}

func TestStatusType_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		dest    StatusType
		wantErr bool
	}{
		{
			name:    "ok",
			src:     `"OK"`,
			dest:    OK,
			wantErr: false,
		},
		{
			name:    "warn",
			src:     `"WARN"`,
			dest:    WARN,
			wantErr: false,
		},
		{
			name:    "error",
			src:     `"ERROR"`,
			dest:    ERROR,
			wantErr: false,
		},
		{
			name:    "chdpc",
			src:     `"chdpc"`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var res StatusType
			if err := res.UnmarshalJSON([]byte(tt.src)); (err != nil) != tt.wantErr {
				t.Errorf("StatusType.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(res, tt.dest) {
				t.Errorf("StatusType.UnmarshalJSON() want = %v, got %v", tt.dest, res)
			}
		})
	}
}
