package middleware

import (
	"bytes"
	"io"
	"net/http"
)

// body holds a (possibly truncated) copy of a request or response body.
type body struct {
	data      []byte
	truncated bool
}

// replayBody lets the handler read a request body which has already been partially consumed for logging.
type replayBody struct {
	io.Reader
	io.Closer
}

// captureRequestBody reads up to limit bytes of the request body and puts them back
// in front of the original body so the handler can read the request as usual.
// Only the copy kept for logging is limited - the handler still sees the whole body.
func captureRequestBody(request *http.Request, limit int) *body {
	if request.Body == nil || request.Body == http.NoBody || limit <= 0 {
		return nil
	}

	original := request.Body

	data, err := io.ReadAll(io.LimitReader(original, int64(limit)+1))
	request.Body = replayBody{
		Reader: io.MultiReader(bytes.NewReader(data), original),
		Closer: original,
	}

	if err != nil || len(data) == 0 {
		return nil
	}

	truncated := len(data) > limit
	if truncated {
		data = data[:limit]
	}

	return &body{
		data:      data,
		truncated: truncated,
	}
}
