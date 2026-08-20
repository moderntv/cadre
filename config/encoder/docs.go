// Package encoder defines how a configuration payload is encoded and decoded.
//
// An [Encoder] is a symmetric pair of Encode and Decode, handed to a
// [github.com/moderntv/cadre/config/source.Source] so that the source itself stays format-agnostic.
//
// The sub-packages provide the built-in implementations: json and yaml. Implement Encoder to support another
// format.
package encoder
