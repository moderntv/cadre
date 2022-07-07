package json

import (
	"encoding/json"

	"github.com/moderntv/cadre/config/encoder"
)

var _ encoder.Encoder = &JsonEncoder{}

type JsonEncoder struct{}

func NewEncoder() (ye *JsonEncoder) {
	return &JsonEncoder{}
}

func (ye *JsonEncoder) Encode(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

func (ye *JsonEncoder) Decode(data []byte, dst interface{}) error {
	return json.Unmarshal(data, dst)
}
