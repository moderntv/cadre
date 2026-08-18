package middleware

import (
	"bytes"
	"net/http"

	"github.com/gin-gonic/gin"
)

// bodyWriter is a gin.ResponseWriter which keeps a copy of the first limit bytes written to it.
type bodyWriter struct {
	gin.ResponseWriter

	limit int
	// contentTypeAllowed decides - once the response content type is known - whether to capture at all.
	contentTypeAllowed func(contentType string) bool

	buffer bytes.Buffer
	total  int

	captureDecided bool
	skipCapture    bool
}

func (bw *bodyWriter) Write(data []byte) (int, error) {
	if bw.shouldCapture() {
		bw.total += len(data)
		bw.buffer.Write(data[:bw.room(len(data))])
	}

	return bw.ResponseWriter.Write(data)
}

func (bw *bodyWriter) WriteString(data string) (int, error) {
	if bw.shouldCapture() {
		bw.total += len(data)
		bw.buffer.WriteString(data[:bw.room(len(data))])
	}

	return bw.ResponseWriter.WriteString(data)
}

// Unwrap gives http.ResponseController access to the wrapped writer - see http.NewResponseController.
func (bw *bodyWriter) Unwrap() http.ResponseWriter {
	return bw.ResponseWriter
}

// room reports how many of the n bytes about to be written still fit into the capture buffer.
func (bw *bodyWriter) room(n int) int {
	return max(min(n, bw.limit-bw.buffer.Len()), 0)
}

// shouldCapture reports whether this response body is eligible for logging.
// The header check is deferred until the first write - the handler sets the headers after the middleware
// runs - and is made only once so that a handler changing them mid-response cannot splice the copy.
// Writes past the limit are still counted, hence no buffer check here - see room and body.
func (bw *bodyWriter) shouldCapture() bool {
	if !bw.captureDecided {
		bw.captureDecided = true
		bw.skipCapture = !bw.contentTypeAllowed(bw.Header().Get("Content-Type")) ||
			isEncoded(bw.Header().Get("Content-Encoding"))
	}

	return !bw.skipCapture
}

func (bw *bodyWriter) body() *body {
	if bw.skipCapture || bw.buffer.Len() == 0 {
		return nil
	}

	return &body{
		data:      bw.buffer.Bytes(),
		truncated: bw.total > bw.limit,
	}
}
