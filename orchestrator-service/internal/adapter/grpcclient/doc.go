// Package grpcclient holds driven adapters that call other services over gRPC.
// Each client implements a port from internal/saga (participants.go) and maps
// saga domain types to and from the contracts/<svc>/v1 protos.
package grpcclient
