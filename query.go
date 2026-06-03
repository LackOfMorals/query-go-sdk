package query

import (
	"context"
	"log/slog"
	"time"

	"github.com/LackOfMorals/query-go-sdk/internal/api"
	"github.com/LackOfMorals/query-go-sdk/internal/decode"
)

// ============================================================================
// Types
// ============================================================================

// instanceService handles instance operations.
type queryService struct {
	api     api.RequestService
	timeout time.Duration
	logger  *slog.Logger
}

// ============================================================================
// Service
// ============================================================================

// Executues a Cypher statement
func (q *queryService) Execute(ctx context.Context, qry string, qryParams map[string]string) (*decode.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, q.timeout)
	defer cancel()

	q.logger.DebugContext(ctx, "running query")

	resp, err := q.api.Post(ctx, `{"statement": "MATCH (p:Person)-[a:ACTED_IN]-(m:Movie {title:'The Matrix'}) RETURN p.name, m.title, a.role"}"`)
	if err != nil {
		q.logger.ErrorContext(ctx, "failed to query", slog.String("error", err.Error()))
		return nil, err
	}

	result, err := decode.DecodeResponse(resp.Body)
	if err != nil {
		q.logger.ErrorContext(ctx, "failed to decode response", slog.String("error", err.Error()))
		return nil, err
	}

	return result, nil
}
