// Package query provides a Go client library for the Neo4j Query API.
package query

import "context"

// QueryService defines operations for using the Query API
type QueryService interface {
	// List returns all tenants accessible to the authenticated user
	Query(ctx context.Context, qry string, qryParams map[string]string) (*QueryResponse, error)
}

// Compile-time interface compliance checks
var (
	_ QueryService = (*queryService)(nil)
)
