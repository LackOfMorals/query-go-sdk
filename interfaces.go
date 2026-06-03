// Package query provides a Go client library for the Neo4j Query API.
package query

import (
	"context"

	"github.com/LackOfMorals/query-go-sdk/internal/decode"
)

// QueryService defines operations for using the Query API
type QueryService interface {
	// Runs a Cypher statement
	Execute(ctx context.Context, qry string, qryParams map[string]string) (*decode.Response, error)
}

// Compile-time interface compliance checks
var (
	_ QueryService = (*queryService)(nil)
)
