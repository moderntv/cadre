package encoder

// Encoder is responsible for encoding/decoding values read from source.
type Encoder interface {
	Encode(data any) ([]byte, error)
	Decode(data []byte, destination any) error
}
