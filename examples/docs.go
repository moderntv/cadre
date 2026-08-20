// Package examples holds the gRPC service implementations shared by the example commands in the
// subdirectories of this module.
//
// Each subdirectory is a runnable command demonstrating one Cadre server topology - httponly, grpconly,
// http-grpc, http-grpc-multiplex, separate-metrics-status and full - plus cli, a small gRPC client for the
// greeter service the others expose. Run one with, for example:
//
//	go run ./httponly
//
// The module is wired to the parent with a replace directive, so the examples always build against the
// working tree rather than a published version.
package examples
