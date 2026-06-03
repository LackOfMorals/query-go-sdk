package query

import (
	"context"
	"encoding/json"
	"fmt"
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

// queryRequest is the JSON body sent to the Query API.
type queryRequest struct {
	Statement  string         `json:"statement"`
	Parameters map[string]any `json:"parameters,omitempty"`
}

// ============================================================================
// Service
// ============================================================================

// Executues a Cypher statement
func (q *queryService) Execute(ctx context.Context, qry string, qryParams map[string]any) (*decode.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, q.timeout)
	defer cancel()

	q.logger.DebugContext(ctx, "running query")

	// Build request body.
	bodyMarshalled, err := json.Marshal(queryRequest{
		Statement:  qry,
		Parameters: qryParams,
	})
	if err != nil {
		return nil, fmt.Errorf("query: marshal request: %w", err)
	}

	// For now, convert back to a string
	body := fmt.Sprintf("%s", bodyMarshalled)

	// cypherQry := `{"statement": "MATCH (p:Person)-[a:ACTED_IN]-(m:Movie {title:'The Matrix'}) RETURN p.name, m.title, a.roles"}"`
	// cypherQry := `{"statement": "MATCH (p)-[a]-(m) RETURN * LIMIT 5"}"`

	resp, err := q.api.Post(ctx, body)
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
