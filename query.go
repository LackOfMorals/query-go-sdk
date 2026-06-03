package query

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/neo4j-contrib/aura-go-sdk/v2/internal/api"
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

// List returns all instances accessible to the authenticated user.
func (q *queryService) Query(ctx context.Context, qry string, qryParams map[string]string) (*QueryResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, q.timeout)
	defer cancel()

	q.logger.DebugContext(ctx, "listing instances")

	resp, err := q.api.Get(ctx, "instances")
	if err != nil {
		q.logger.ErrorContext(ctx, "failed to query", slog.String("error", err.Error()))
		return nil, err
	}

	var result QueryResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		q.logger.ErrorContext(ctx, "failed to unmarshal instances response", slog.String("error", err.Error()))
		return nil, err
	}

	q.logger.DebugContext(ctx, "instances listed successfully", slog.Int("count", len(result.Data)))
	return &result, nil
}
