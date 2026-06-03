package query

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/LackOfMorals/query-go-sdk/internal/api"
)

// ============================================================================
// Types
// ============================================================================

// ListInstancesResponse contains a list of instances in a tenant.
type QueryResponse struct {
	Data []QueryData `json:"data"`
}

// QueryData holds the summary fields returned for each instance in a list response.
type QueryData struct {
}

// ============================================================================
// Service
// ============================================================================

// instanceService handles instance operations.
type queryService struct {
	api     api.RequestService
	timeout time.Duration
	logger  *slog.Logger
}

// Executues a Cypher statement
func (q *queryService) Execute(ctx context.Context, qry string, qryParams map[string]string) (*QueryResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, q.timeout)
	defer cancel()

	q.logger.DebugContext(ctx, "running query")

	resp, err := q.api.Post(ctx, `{"statement": "MATCH (p:Person)-[a:ACTED_IN]-(m:Movie {title:'The Matrix'}) RETURN p.name, m.title, a.role"}"`)
	if err != nil {
		q.logger.ErrorContext(ctx, "failed to query", slog.String("error", err.Error()))
		return nil, err
	}

	var result QueryResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		q.logger.ErrorContext(ctx, "failed to unmarshal query response", slog.String("error", err.Error()))
		return nil, err
	}

	q.logger.DebugContext(ctx, "query ran successfully", slog.Int("count", len(result.Data)))
	return &result, nil
}
