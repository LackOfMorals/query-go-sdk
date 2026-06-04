// Package query provides a Go client library for the Neo4j Query API.
// and is based on the aura-go-sdk
//
// The client supports all major Query API operations including
// - query with or without parameters
// - use of plain JSON  or JSON with types formats
// - explicit transactions
//
// Example usage:
//
//	client, err := query.NewClient(
//	    query.WithBasicCredentials("username", "password"),
//	    query.WithURL("http://localhost:7474"),
//	    query.WithDB("neo4j"),
//	    query.WithFormat("JSON"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// result := client.query("MATCH (n) RETURN * LIMIT 1","")

package query

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/LackOfMorals/query-go-sdk/internal/api"
)

// ============================================================================
// Constants and version
// ============================================================================

// neo4jVersion is the min version of Neo4jthis client targets.
// Query API has evolved over time and , whilst avoiding breaking changes, there
// are differences such that a min version is necessary

const neo4jVersion = ""

// clientVersionFallback is embedded in the User-Agent when the real module version cannot
// be determined (local builds, go test, go run). It is intentionally kept as "development"
// in source — there is no need to update it before tagging a release.
const clientVersionFallback = "development"

// ClientVersion is the version of this client library, embedded in every User-Agent header.
//
// Why debug.ReadBuildInfo()?
// Go consumers import this library by source (via the module proxy). There are no compiled
// binaries to stamp at build time. When a consumer builds their application, the Go toolchain
// records all module dependencies and their exact versions in the binary. debug.ReadBuildInfo()
// reads that information at runtime, so the User-Agent automatically reflects the version the
// consumer actually imported (e.g. "v1.10.0") without any source edits or workflow tricks.
//
// In local and test builds, ReadBuildInfo returns "(devel)" or fails entirely, so we fall back
// to clientVersionFallback ("development") to make it obvious the binary is not a release build.
var ClientVersion = clientVersionFallback

func init() {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		ClientVersion = info.Main.Version
	}
}

// ============================================================================
// Client types
// ============================================================================

// QueryAPIClient is the main client for interacting with the Neo4j Aura API.
//
//nolint:revive // QueryAPIClient is intentional: the package is named query and the type name is established in v1.
type QueryAPIClient struct {
	api    api.RequestService // Handles authenticated API requests
	logger *slog.Logger       // Structured logger

	// Grouped services — using interface types for testability.
	Query QueryService
}

// config holds internal configuration (unexported).
type config struct {
	baseURL         string            // the base URL of neo4j server
	apiTimeout      time.Duration     // how long to wait for a response from an Aura API endpoint
	apiRetryMax     int               // the number of retries to attempt
	authHeader      api.Credentials   // The auth header value to use
	database        string            // database
	httpClient      *http.Client      // optional custom HTTP client (injected transport)
	userAgent       string            // optional User-Agent override
	defaultHeaders  map[string]string // optional headers added to every API request
	maxResponseSize int               // optional max response size in bytes
	clientVersion   string            // the version of this query client
}

// Option is a functional option for configuring the AuraAPIClient.
type Option func(*options) error

// options holds the configuration that will be applied to the client.
type options struct {
	config config
	logger *slog.Logger
}

// ============================================================================
// Constructor and options
// ============================================================================

// defaultOptions returns options with sensible defaults.
func defaultOptions() *options {
	opts := &slog.HandlerOptions{Level: slog.LevelWarn}
	handler := slog.NewTextHandler(os.Stderr, opts)

	return &options{
		config: config{
			baseURL:         "http://localhost:7474",
			database:        "neo4j",
			apiTimeout:      120 * time.Second,
			apiRetryMax:     3,
			clientVersion:   ClientVersion,
			userAgent:       "query-go-sdk/" + ClientVersion,
			maxResponseSize: 10 * 1024 * 1024, // This is 10mb
		},
		logger: slog.New(handler),
	}
}

func WithBasicAuth(username, password string) Option {
	return func(o *options) error {
		if o.config.authHeader != nil {
			return errors.New("auth already set: WithBasicAuth and WithBearerToken are mutually exclusive")
		}
		o.config.authHeader = &api.BasicCredentials{Username: username, Password: password}
		return nil
	}
}

func WithBearerToken(token string) Option {
	return func(o *options) error {
		if o.config.authHeader != nil {
			return errors.New("auth already set: WithBasicAuth and WithBearerToken are mutually exclusive")
		}
		o.config.authHeader = &api.StaticCredentials{Token: token}
		return nil
	}
}

// WithDatabase
func WithDatabase(database string) Option {
	return func(o *options) error {
		// check database is not empty
		if database == "" {
			return errors.New("database must not be empty")
		}
		o.config.database = database
		return nil
	}
}

// WithTimeout sets a custom API timeout. Defaults to 120 seconds.
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) error {
		if timeout <= 0 {
			return errors.New("timeout must be greater than zero")
		}
		o.config.apiTimeout = timeout
		return nil
	}
}

// WithMaxRetry sets the maximum number of retries for failed requests. Defaults to 3.
func WithMaxRetry(maxRetry int) Option {
	return func(o *options) error {
		if maxRetry <= 0 {
			return errors.New("max retries must be greater than zero")
		}
		o.config.apiRetryMax = maxRetry
		return nil
	}
}

// WithMaxResponseSize sets the maximum size for  response.  Default is 10mb
func WithMaxResponseSize(maxResponse int) Option {
	return func(o *options) error {
		if maxResponse <= 0 {
			return errors.New("max response size must be greater than zero")
		}
		o.config.maxResponseSize = maxResponse
		return nil
	}
}

// WithLogger sets a custom slog.Logger. Defaults to warn-level logging to stderr.
func WithLogger(logger *slog.Logger) Option {
	return func(o *options) error {
		if logger == nil {
			return errors.New("logger cannot be nil")
		}
		o.logger = logger
		return nil
	}
}

// WithBaseURL overrides the default URL.
func WithBaseURL(baseURL string) Option {
	return func(o *options) error {
		if baseURL == "" {
			return errors.New("base URL must not be empty")
		}
		/* Might enforce this for Aura DBs if we can figure out the URL.
		if !strings.HasPrefix(baseURL, "https://") {
			return errors.New("base URL must use HTTPS to protect credentials in transit")
		}
		*/

		o.config.baseURL = baseURL
		return nil
	}
}

// WithHTTPClient sets a custom *http.Client to use for all API requests. This
// lets callers inject a custom transport (e.g. for mTLS, proxies, or testing).
// Returns an error if client is nil.
func WithHTTPClient(client *http.Client) Option {
	return func(o *options) error {
		if client == nil {
			return errors.New("HTTP client cannot be nil")
		}
		o.config.httpClient = client
		return nil
	}
}

// WithUserAgent overrides the default User-Agent header sent with every request.
// Returns an error if ua is empty.
func WithUserAgent(ua string) Option {
	return func(o *options) error {
		if ua == "" {
			return errors.New("user agent must not be empty")
		}
		o.config.userAgent = ua
		return nil
	}
}

// protectedHeaders is the set of header keys that WithDefaultHeaders silently
// drops to prevent callers from inadvertently overriding security-sensitive or
// protocol-critical headers.
var protectedHeaders = map[string]struct{}{
	"authorization": {},
	"content-type":  {},
	"accept":        {},
	"user-agent":    {},
}

// WithDefaultHeaders adds the given headers to every API request. It is a no-op
// when headers is nil or empty. Keys matching Authorization, Content-Type, or
// User-Agent (case-insensitive) are silently ignored to protect credentials and
// protocol semantics.
func WithDefaultHeaders(headers map[string]string) Option {
	return func(o *options) error {
		if len(headers) == 0 {
			return nil
		}
		filtered := make(map[string]string, len(headers))
		for k, v := range headers {
			if _, protected := protectedHeaders[strings.ToLower(k)]; !protected {
				filtered[k] = v
			}
		}
		if len(filtered) > 0 {
			o.config.defaultHeaders = filtered
		}
		return nil
	}
}

// Close drains idle HTTP connections held by the underlying transport. It
// should be called via defer when the client is no longer needed to avoid
// leaking file descriptors.
//
//	client, err := query.NewClient(query.WithBasicAuth("neo4j", "password"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
func (c *QueryAPIClient) Close() {
	c.api.Close()
}

// NewClient creates a new Query API client with functional options.
func NewClient(opts ...Option) (*QueryAPIClient, error) {
	// set the default options.  These will be overridden where this is a supplied option
	o := defaultOptions()

	for _, opt := range opts {
		if err := opt(o); err != nil {
			o.logger.Error("option application failed", slog.String("error", err.Error()))
			return nil, err
		}
	}

	if o.config.authHeader == nil {
		o.logger.Error("validation failed", slog.String("reason", "username/ password or Token must be given"))
		return nil, errors.New("username must not be empty")
	}

	if o.config.baseURL == "" {
		o.logger.Error("validation failed", slog.String("reason", "base URL must not be empty"))
		return nil, errors.New("base URL must not be empty")
	}
	if o.config.apiTimeout <= 0 {
		o.logger.Error("validation failed", slog.String("reason", "API timeout must be greater than zero"), slog.Duration("timeout", o.config.apiTimeout))
		return nil, errors.New("API timeout must be greater than zero")
	}

	// Technically the user agent could be empty. Our usage analysis relies on this being set so
	// we don't allow it to be empty
	// Custom userAgent maybe withdrawn.
	if o.config.userAgent == "" {
		o.logger.Error("validation failed", slog.String("reason", "User agent cannot be empty"))
		return nil, errors.New("user agent cannot be empty")
	}

	o.logger.Debug("configuration validated",
		slog.String("baseURL", o.config.baseURL),
		slog.String("apiVersion", ClientVersion),
		slog.Duration("apiTimeout", o.config.apiTimeout),
	)

	apiSvc := api.NewRequestService(api.Config{
		AuthHeader:      o.config.authHeader,
		BaseURL:         o.config.baseURL,
		Database:        o.config.database,
		ClientVersion:   ClientVersion,
		Timeout:         o.config.apiTimeout,
		MaxRetry:        o.config.apiRetryMax,
		UserAgent:       o.config.userAgent,
		HTTPClient:      o.config.httpClient,
		DefaultHeaders:  o.config.defaultHeaders,
		MaxResponseSize: o.config.maxResponseSize,
	}, o.logger)

	clientLogger := o.logger.With(slog.String("component", "QueryAPIClient"))

	service := &QueryAPIClient{
		api:    apiSvc,
		logger: clientLogger,
	}

	service.Query = &queryService{
		api:     apiSvc,
		timeout: o.config.apiTimeout,
		logger:  clientLogger.With(slog.String("service", "queryService")),
	}

	service.logger.Info("Query API client initialized successfully",
		slog.String("sdk version", ClientVersion),
	)

	return service, nil
}
