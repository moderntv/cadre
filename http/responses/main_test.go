package responses

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

func TestNewErrors(t *testing.T) {
	type args struct {
		errs []error
	}
	tests := []struct {
		name string
		args args
		want []Error
	}{
		{
			name: "simple",
			args: args{
				errs: []error{errors.New("simple1"), errors.New("simple2")},
			},
			want: []Error{
				{
					Type:    "GENERIC_ERROR",
					Message: "Error encountered",
					Data:    "simple1",
				},
				{
					Type:    "GENERIC_ERROR",
					Message: "Error encountered",
					Data:    "simple2",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewErrors(tt.args.errs...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewErrors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewError(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want Error
	}{
		{
			name: "simple",
			args: args{
				err: errors.New("simple"),
			},
			want: Error{
				Type:    "GENERIC_ERROR",
				Message: "Error encountered",
				Data:    "simple",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewError(tt.args.err); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewError() = %v, want %v", got, tt.want)
			}
		})
	}
}

type httpTestShit struct {
	recorder   *httptest.ResponseRecorder
	ginContext *gin.Context
}

func newHttpTestshit() httpTestShit {
	r := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(r)

	return httpTestShit{r, c}
}

func TestOk(t *testing.T) {
	tests := []struct {
		name         string
		httpTestShit httpTestShit
		data         any
		wantStatus   int
		wantBody     string
	}{
		{
			name:         "empty",
			httpTestShit: newHttpTestshit(),
			data:         nil,
			wantStatus:   http.StatusOK,
			wantBody:     `{"data":null}`,
		},
		{
			name:         "string",
			httpTestShit: newHttpTestshit(),
			data:         "foobar",
			wantStatus:   http.StatusOK,
			wantBody:     `{"data":"foobar"}`,
		},
		{
			name:         "map",
			httpTestShit: newHttpTestshit(),
			data:         map[string]any{"a": "b", "c": 4, "e": 6.01, "g": false},
			wantStatus:   http.StatusOK,
			wantBody:     `{"data":{"a":"b","c":4,"e":6.01,"g":false}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Ok(tt.httpTestShit.ginContext, tt.data)

			res := tt.httpTestShit.recorder.Result()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("Ok response invalid status code. want %d, got %v", tt.wantStatus, res.StatusCode)
			}
			b := tt.httpTestShit.recorder.Body.String()
			if b != tt.wantBody {
				t.Errorf("Ok response has unexpected body. want `%v`, got `%v`", tt.wantBody, b)
			}
		})
	}
}

func TestOkWithMeta(t *testing.T) {
	tests := []struct {
		name         string
		httpTestShit httpTestShit
		data         any
		metadata     any
		wantStatus   int
		wantBody     string
	}{
		{
			name:         "simple",
			httpTestShit: newHttpTestshit(),
			data:         map[string]any{"a": "b", "c": 4, "e": 6.01, "g": false},
			metadata:     map[string]any{"lorem": "ipsum"},
			wantStatus:   http.StatusOK,
			wantBody:     `{"data":{"a":"b","c":4,"e":6.01,"g":false},"metadata":{"lorem":"ipsum"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			OkWithMeta(tt.httpTestShit.ginContext, tt.data, tt.metadata)

			res := tt.httpTestShit.recorder.Result()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("OkWithMeta response invalid status code. want %d, got %v", tt.wantStatus, res.StatusCode)
			}
			b := tt.httpTestShit.recorder.Body.String()
			if b != tt.wantBody {
				t.Errorf("OkWithMeta response has unexpected body. want `%v`, got `%v`", tt.wantBody, b)
			}
		})
	}
}

func TestCreated(t *testing.T) {
	tests := []struct {
		name         string
		httpTestShit httpTestShit
		data         any
		wantStatus   int
		wantBody     string
	}{
		{
			name:         "simple",
			httpTestShit: newHttpTestshit(),
			data:         map[string]any{"a": "b", "c": 4, "e": 6.01, "g": false},
			wantStatus:   http.StatusCreated,
			wantBody:     `{"data":{"a":"b","c":4,"e":6.01,"g":false}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Created(tt.httpTestShit.ginContext, tt.data)

			res := tt.httpTestShit.recorder.Result()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("Created response invalid status code. want %d, got %d", tt.wantStatus, res.StatusCode)
			}
			b := tt.httpTestShit.recorder.Body.String()
			if b != tt.wantBody {
				t.Errorf("Created response has unexpected body. want `%v`, got `%v`", tt.wantBody, b)
			}
		})
	}
}

func TestBadRequest(t *testing.T) {
	tests := []struct {
		name         string
		httpTestShit httpTestShit
		errors       []error
		wantStatus   int
		wantBody     string
	}{
		{
			name:         "simple",
			httpTestShit: newHttpTestshit(),
			errors:       []error{errors.New("error1"), errors.New("error2")},
			wantStatus:   http.StatusBadRequest,
			wantBody:     `{"message":"The request is not valid in this context","errors":[{"type":"GENERIC_ERROR","message":"Error encountered","data":"error1"},{"type":"GENERIC_ERROR","message":"Error encountered","data":"error2"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			BadRequest(tt.httpTestShit.ginContext, NewErrors(tt.errors...)...)

			res := tt.httpTestShit.recorder.Result()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("Created response invalid status code. want %d, got %d", tt.wantStatus, res.StatusCode)
			}
			b := tt.httpTestShit.recorder.Body.String()
			if b != tt.wantBody {
				t.Errorf("Created response has unexpected body. want `%v`, got `%v`", tt.wantBody, b)
			}
		})
	}
}

func TestCannotBind(t *testing.T) {
	tests := []struct {
		name         string
		httpTestShit httpTestShit
		err          error
		wantStatus   int
		wantBody     string
	}{
		{
			name:         "empty",
			httpTestShit: newHttpTestshit(),
			err:          nil,
			wantStatus:   http.StatusBadRequest,
			wantBody:     `{"message":"The request is not valid in this context","errors":[{"type":"UNKNOWN_INPUT_VALIDATION_ERROR","message":"Sent data do not correspond to the template","data":""}]}`,
		},
		{
			name:         "empty",
			httpTestShit: newHttpTestshit(),
			err:          errors.New("error1"),
			wantStatus:   http.StatusBadRequest,
			wantBody:     `{"message":"The request is not valid in this context","errors":[{"type":"UNKNOWN_INPUT_VALIDATION_ERROR","message":"There were errors when applying sent data to template: error1","data":"error1"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			CannotBind(tt.httpTestShit.ginContext, tt.err)

			res := tt.httpTestShit.recorder.Result()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("Created response invalid status code. want %d, got %d", tt.wantStatus, res.StatusCode)
			}
			b := tt.httpTestShit.recorder.Body.String()
			if b != tt.wantBody {
				t.Errorf("Created response has unexpected body. want `%v`, got `%v`", tt.wantBody, b)
			}
		})
	}
}

func TestUnauthorized(t *testing.T) {
	tests := []struct {
		name         string
		httpTestShit httpTestShit
		errors       []error
		wantStatus   int
		wantBody     string
	}{
		{
			name:         "simple",
			httpTestShit: newHttpTestshit(),
			errors:       []error{errors.New("error1")},
			wantStatus:   http.StatusUnauthorized,
			wantBody:     `{"message":"You have to be logged in to view this resource","errors":[{"type":"GENERIC_ERROR","message":"Error encountered","data":"error1"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Unauthorized(tt.httpTestShit.ginContext, NewErrors(tt.errors...)...)

			res := tt.httpTestShit.recorder.Result()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("Created response invalid status code. want %d, got %d", tt.wantStatus, res.StatusCode)
			}
			b := tt.httpTestShit.recorder.Body.String()
			if b != tt.wantBody {
				t.Errorf("Created response has unexpected body. want `%v`, got `%v`", tt.wantBody, b)
			}
		})
	}
}

func TestForbidden(t *testing.T) {
	tests := []struct {
		name         string
		httpTestShit httpTestShit
		errors       []error
		wantStatus   int
		wantBody     string
	}{
		{
			name:         "simple",
			httpTestShit: newHttpTestshit(),
			errors:       []error{errors.New("error1")},
			wantStatus:   http.StatusForbidden,
			wantBody:     `{"message":"You are not allowed to view this resource","errors":[{"type":"GENERIC_ERROR","message":"Error encountered","data":"error1"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Forbidden(tt.httpTestShit.ginContext, NewErrors(tt.errors...)...)

			res := tt.httpTestShit.recorder.Result()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("Created response invalid status code. want %d, got %d", tt.wantStatus, res.StatusCode)
			}
			b := tt.httpTestShit.recorder.Body.String()
			if b != tt.wantBody {
				t.Errorf("Created response has unexpected body. want `%v`, got `%v`", tt.wantBody, b)
			}
		})
	}
}

func TestNotFound(t *testing.T) {
	tests := []struct {
		name         string
		httpTestShit httpTestShit
		errors       []error
		wantStatus   int
		wantBody     string
	}{
		{
			name:         "simple",
			httpTestShit: newHttpTestshit(),
			errors:       []error{errors.New("error1")},
			wantStatus:   http.StatusNotFound,
			wantBody:     `{"message":"The resource is unavailable","errors":[{"type":"GENERIC_ERROR","message":"Error encountered","data":"error1"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NotFound(tt.httpTestShit.ginContext, NewErrors(tt.errors...)...)

			res := tt.httpTestShit.recorder.Result()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("Created response invalid status code. want %d, got %d", tt.wantStatus, res.StatusCode)
			}
			b := tt.httpTestShit.recorder.Body.String()
			if b != tt.wantBody {
				t.Errorf("Created response has unexpected body. want `%v`, got `%v`", tt.wantBody, b)
			}
		})
	}
}

func TestTimeout(t *testing.T) {
	tests := []struct {
		name         string
		httpTestShit httpTestShit
		errors       []error
		wantStatus   int
		wantBody     string
	}{
		{
			name:         "simple",
			httpTestShit: newHttpTestshit(),
			errors:       []error{errors.New("error1")},
			wantStatus:   http.StatusRequestTimeout,
			wantBody:     `{"message":"Request timed out"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Timeout(tt.httpTestShit.ginContext)

			res := tt.httpTestShit.recorder.Result()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("Created response invalid status code. want %d, got %d", tt.wantStatus, res.StatusCode)
			}
			b := tt.httpTestShit.recorder.Body.String()
			if b != tt.wantBody {
				t.Errorf("Created response has unexpected body. want `%v`, got `%v`", tt.wantBody, b)
			}
		})
	}
}

func TestConflict(t *testing.T) {
	tests := []struct {
		name         string
		httpTestShit httpTestShit
		errors       []error
		wantStatus   int
		wantBody     string
	}{
		{
			name:         "simple",
			httpTestShit: newHttpTestshit(),
			errors:       []error{errors.New("error1")},
			wantStatus:   http.StatusConflict,
			wantBody:     `{"message":"Cannot complete due to a conflict","errors":[{"type":"GENERIC_ERROR","message":"Error encountered","data":"error1"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Conflict(tt.httpTestShit.ginContext, NewErrors(tt.errors...)...)

			res := tt.httpTestShit.recorder.Result()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("Created response invalid status code. want %d, got %d", tt.wantStatus, res.StatusCode)
			}
			b := tt.httpTestShit.recorder.Body.String()
			if b != tt.wantBody {
				t.Errorf("Created response has unexpected body. want `%v`, got `%v`", tt.wantBody, b)
			}
		})
	}
}

func TestUnavailable(t *testing.T) {
	tests := []struct {
		name         string
		httpTestShit httpTestShit
		errors       []error
		wantStatus   int
		wantBody     string
	}{
		{
			name:         "simple",
			httpTestShit: newHttpTestshit(),
			errors:       []error{errors.New("error1")},
			wantStatus:   http.StatusServiceUnavailable,
			wantBody:     `{"message":"Service is temporarily unavailable","errors":[{"type":"GENERIC_ERROR","message":"Error encountered","data":"error1"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Unavailable(tt.httpTestShit.ginContext, NewErrors(tt.errors...)...)

			res := tt.httpTestShit.recorder.Result()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("Created response invalid status code. want %d, got %d", tt.wantStatus, res.StatusCode)
			}
			b := tt.httpTestShit.recorder.Body.String()
			if b != tt.wantBody {
				t.Errorf("Created response has unexpected body. want `%v`, got `%v`", tt.wantBody, b)
			}
		})
	}
}

func TestInternalError(t *testing.T) {
	tests := []struct {
		name         string
		httpTestShit httpTestShit
		errors       []error
		wantStatus   int
		wantBody     string
	}{
		{
			name:         "simple",
			httpTestShit: newHttpTestshit(),
			errors:       []error{errors.New("error1")},
			wantStatus:   http.StatusInternalServerError,
			wantBody:     `{"message":"An unexpected error has occurred. A team of monkeys was already sent to site. We're not sure, when it will be ready, but it sure as hell will be banana","errors":[{"type":"GENERIC_ERROR","message":"Error encountered","data":"error1"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InternalError(tt.httpTestShit.ginContext, NewErrors(tt.errors...)...)

			res := tt.httpTestShit.recorder.Result()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("Created response invalid status code. want %d, got %d", tt.wantStatus, res.StatusCode)
			}

			assert.Equal(t, tt.wantBody, tt.httpTestShit.recorder.Body.String())
		})
	}
}
