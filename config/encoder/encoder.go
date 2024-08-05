package encoder

// Encoder is responsible for encoding/decoding values read from source.
type Encoder interface {
	Encode(interface{}) ([]byte, error)
	Decode([]byte, interface{}) error
}
