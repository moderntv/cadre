package status

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type StatusType int

const (
	ERROR StatusType = iota
	WARN
	OK
)

func (s *StatusType) String() string {
	return toString[*s]
}

var toString = map[StatusType]string{
	OK:    "OK",
	WARN:  "WARN",
	ERROR: "ERROR",
}

var toID = map[string]StatusType{
	"OK":    OK,
	"WARN":  WARN,
	"ERROR": ERROR,
}

func (s *StatusType) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(toString[*s])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

func (s *StatusType) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}

	t, ok := toID[j]
	if !ok {
		return fmt.Errorf("invalid StatusType: %v", string(b))
	}

	*s = t
	return nil
}
