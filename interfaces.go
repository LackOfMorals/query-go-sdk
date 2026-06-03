// Package query provides a Go client library for the Neo4j Query API.
package query

import (
	"context"

	"github.com/LackOfMorals/query-go-sdk/internal/decode"
)

// QueryService defines operations for using the Query API
type QueryService interface {
	// Runs a Cypher statement
	Execute(ctx context.Context, qry string, qryParams map[string]any) (*decode.Response, error)
}

// Compile-time interface compliance checks
var (
	_ QueryService = (*queryService)(nil)
)

// Re-export error types from internal/decode so callers never need to import
// internal packages directly.

// QueryErrors is returned when Neo4j responds with one or more errors.
// Inspect with errors.As:
//
//	var qErr *query.QueryErrors
//	if errors.As(err, &qErr) {
//	    for _, e := range qErr.Errors {
//	        log.Printf("[%s] %s", e.Title(), e.Message)
//	    }
//	}
type QueryErrors = decode.QueryErrors

// QueryError is a single Neo4j error within a QueryErrors batch.
// Use Classification(), Category(), and Title() to branch on the error code.
type QueryError = decode.QueryError
