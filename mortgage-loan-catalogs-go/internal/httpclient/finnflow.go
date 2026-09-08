package httpclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"mortgage-loan-catalogs-go/internal/catalog"
)

// NoResultsResponse is an alias to catalog.NoResultsResponse.
type NoResultsResponse = catalog.NoResultsResponse

// HTTPError represents an HTTP error that carries a status code and error details.
type HTTPError struct {
	StatusCode   int    `json:"status_code"`
	ErrorsDetail string `json:"errors_detail"`
	RawBody      []byte `json:"-"`
}

func (e *HTTPError) Error() string {
	return e.ErrorsDetail
}

// HTTPStatus returns the HTTP status code.
func (e *HTTPError) HTTPStatus() int {
	return e.StatusCode
}

// StatusCode returns the HTTP status code.
func (e *HTTPError) StatusCodeVal() int {
	return e.StatusCode
}

// Status returns the HTTP status code.
func (e *HTTPError) Status() int {
	return e.StatusCode
}

// NewCatalogNotFoundError creates an error representing catalog not found, translated to HTTP 402.
func NewCatalogNotFoundError() *HTTPError {
	return &HTTPError{
		StatusCode:   http.StatusPaymentRequired, // 402
		ErrorsDetail: "Catálogo no encontrado",
	}
}

// ErrCatalogNotFound is a sentinel error for catalog not found (HTTP 402).
var ErrCatalogNotFound = NewCatalogNotFoundError()

// NewTokenNotValidError creates an error representing invalid token with HTTP 401.
func NewTokenNotValidError() *HTTPError {
	return &HTTPError{
		StatusCode:   http.StatusUnauthorized, // 401
		ErrorsDetail: "Token no válido",
	}
}

// ErrTokenNotValid is a sentinel error for token invalid (HTTP 401).
var ErrTokenNotValid = NewTokenNotValidError()

// NewCatalogError creates an error representing catalog retrieval failure with HTTP 400.
func NewCatalogError(msg string) *HTTPError {
	if msg == "" {
		msg = "Error en la obtención de catálogos"
	}
	return &HTTPError{
		StatusCode:   http.StatusBadRequest, // 400
		ErrorsDetail: msg,
	}
}

// ErrCatalogError is a sentinel error for generic catalog error (HTTP 400).
var ErrCatalogError = NewCatalogError("Error en la obtención de catálogos")

// IsCatalogNotFound checks if an error represents catalog not found (HTTP 402).
func IsCatalogNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrCatalogNotFound) {
		return true
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == http.StatusPaymentRequired
	}
	return false
}

// StatusCodeFromError extracts HTTP status code from an error if possible, defaulting to fallback.
func StatusCodeFromError(err error, fallback int) int {
	if err == nil {
		return http.StatusOK
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode
	}
	type statusCoder interface {
		StatusCode() int
	}
	var sc statusCoder
	if errors.As(err, &sc) {
		return sc.StatusCode()
	}
	type httpStatusCoder interface {
		HTTPStatus() int
	}
	var hsc httpStatusCoder
	if errors.As(err, &hsc) {
		return hsc.HTTPStatus()
	}
	return fallback
}

// Config contains configuration for the Finnflow HTTP client.
type Config struct {
	BaseURL    string
	Username   string
	Password   string
	Timeout    time.Duration
	HTTPClient *http.Client
}

// DefaultConfig builds configuration initialized from standard environment variables:
// FINNFLOW_URL, FINNFLOW_KEY (or FINNFLOW_USERNAME), FINNFLOW_SECRET (or FINNFLOW_PASSWORD), FINNFLOW_TIMEOUT.
func DefaultConfig() Config {
	baseURL := os.Getenv("FINNFLOW_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	username := os.Getenv("FINNFLOW_KEY")
	if username == "" {
		username = os.Getenv("FINNFLOW_USERNAME")
	}

	password := os.Getenv("FINNFLOW_SECRET")
	if password == "" {
		password = os.Getenv("FINNFLOW_PASSWORD")
	}

	timeout := 30 * time.Second
	if timeoutStr := os.Getenv("FINNFLOW_TIMEOUT"); timeoutStr != "" {
		if d, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = d
		}
	}

	return Config{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		Timeout:  timeout,
	}
}

// Client represents the HTTP client communicating with FinnFlow.
type Client struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

// FinnflowClient is an exported alias for Client.
type FinnflowClient = Client

// NewClient creates a new FinnFlow HTTP client using the provided configuration.
func NewClient(cfg Config) *Client {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = os.Getenv("FINNFLOW_URL")
		if baseURL == "" {
			baseURL = "http://localhost:8080"
		}
	}

	username := cfg.Username
	if username == "" {
		username = os.Getenv("FINNFLOW_KEY")
		if username == "" {
			username = os.Getenv("FINNFLOW_USERNAME")
		}
	}

	password := cfg.Password
	if password == "" {
		password = os.Getenv("FINNFLOW_SECRET")
		if password == "" {
			password = os.Getenv("FINNFLOW_PASSWORD")
		}
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: timeout,
		}
	}

	return &Client{
		baseURL:    baseURL,
		username:   username,
		password:   password,
		httpClient: httpClient,
	}
}

// New is an alias for NewClient.
func New(cfg Config) *Client {
	return NewClient(cfg)
}

// NewFromEnv creates a Client initialized with DefaultConfig().
func NewFromEnv() *Client {
	return NewClient(DefaultConfig())
}

// NewFinnflowClient is an alias for NewClient.
func NewFinnflowClient(cfg Config) *FinnflowClient {
	return NewClient(cfg)
}

// BaseURL returns the configured base URL.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// buildURL constructs the endpoint ${FINNFLOW_URL}/api/catalogo_detail/{catalog}.
func (c *Client) buildURL(catalogName string) string {
	base := strings.TrimRight(c.baseURL, "/")
	if !strings.HasSuffix(base, "/api/catalogo_detail") {
		base += "/api/catalogo_detail"
	}
	return fmt.Sprintf("%s/%s", base, url.PathEscape(catalogName))
}

// setHeaders applies application/json headers and Base64 Basic authentication.
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if c.username != "" || c.password != "" {
		auth := c.username + ":" + c.password
		encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
		req.Header.Set("Authorization", "Basic "+encodedAuth)
	}
}

// isNetworkOrTimeoutError inspects an error to determine if it is a network failure,
// socket connection issue, or timeout.
func isNetworkOrTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return true
		}
		var innerOp *net.OpError
		if errors.As(urlErr.Err, &innerOp) {
			return true
		}
		var innerNet net.Error
		if errors.As(urlErr.Err, &innerNet) {
			return true
		}
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connection") ||
		strings.Contains(msg, "conexión") ||
		strings.Contains(msg, "connect") ||
		strings.Contains(msg, "dial") ||
		strings.Contains(msg, "deadline") ||
		strings.Contains(msg, "refused") ||
		strings.Contains(msg, "denegó") ||
		strings.Contains(msg, "reset") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "network is unreachable") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "client.timeout")
}

// executeSingleCall performs EXACTLY 1 outbound HTTP call to FinnFlow, reading the body to []byte.
// It handles network errors / timeouts and status code translation.
func (c *Client) executeSingleCall(ctx context.Context, catalogName string) (bodyBytes []byte, statusCode int, isFallback bool, err error) {
	endpoint := c.buildURL(catalogName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, false, err
	}

	c.setHeaders(req)

	// Optimization: ONLY 1 outbound HTTP call
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// If timeout or network error, return NoResultsResponse with status 200
		if isNetworkOrTimeoutError(err) {
			return nil, http.StatusOK, true, nil
		}
		return nil, 0, false, err
	}
	defer resp.Body.Close()

	// Read body into []byte
	bodyBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		if isNetworkOrTimeoutError(err) {
			return nil, http.StatusOK, true, nil
		}
		return nil, resp.StatusCode, false, err
	}

	// Status code translation
	switch resp.StatusCode {
	case http.StatusOK:
		return bodyBytes, http.StatusOK, false, nil

	case http.StatusNotFound:
		// FinnFlow 404 is translated to client HTTP 402 ("Catálogo no encontrado")
		return bodyBytes, http.StatusPaymentRequired, false, &HTTPError{
			StatusCode:   http.StatusPaymentRequired,
			ErrorsDetail: "Catálogo no encontrado",
			RawBody:      bodyBytes,
		}

	case http.StatusPaymentRequired:
		// Upstream 402 preserved as client HTTP 402
		return bodyBytes, http.StatusPaymentRequired, false, &HTTPError{
			StatusCode:   http.StatusPaymentRequired,
			ErrorsDetail: "Catálogo no encontrado",
			RawBody:      bodyBytes,
		}

	case http.StatusUnauthorized:
		// Upstream 401 translated to client HTTP 401
		return bodyBytes, http.StatusUnauthorized, false, &HTTPError{
			StatusCode:   http.StatusUnauthorized,
			ErrorsDetail: "Token no válido",
			RawBody:      bodyBytes,
		}

	case http.StatusBadRequest:
		// Upstream 400 translated to client HTTP 400
		return bodyBytes, http.StatusBadRequest, false, &HTTPError{
			StatusCode:   http.StatusBadRequest,
			ErrorsDetail: "Error en la obtención de catálogos",
			RawBody:      bodyBytes,
		}

	default:
		if resp.StatusCode >= 400 {
			return bodyBytes, http.StatusBadRequest, false, &HTTPError{
				StatusCode:   http.StatusBadRequest,
				ErrorsDetail: "Error en la obtención de catálogos",
				RawBody:      bodyBytes,
			}
		}
		return bodyBytes, resp.StatusCode, false, nil
	}
}

// isNoResultsBody checks if the body matches the legacy upstream "Sin resultados" pattern.
func isNoResultsBody(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("[]")) || bytes.Equal(trimmed, []byte("null")) {
		return true
	}
	s := string(trimmed)
	return strings.Contains(s, "codRespuesta") && strings.Contains(s, "Sin resultados")
}

// FallbackNoResultsList returns a slice with a default NoResultsResponse matching the Java API contract.
func FallbackNoResultsList() []catalog.NoResultsResponse {
	return []catalog.NoResultsResponse{catalog.NewNoResultsResponse()}
}

// FallbackNoResultsResponse returns the default NoResultsResponse struct.
func FallbackNoResultsResponse() catalog.NoResultsResponse {
	return catalog.NewNoResultsResponse()
}

// AsNoResultsResponse extracts a catalog.NoResultsResponse if the data represents empty/fallback results.
func AsNoResultsResponse(data any) (catalog.NoResultsResponse, bool) {
	switch v := data.(type) {
	case catalog.NoResultsResponse:
		return v, true
	case *catalog.NoResultsResponse:
		if v != nil {
			return *v, true
		}
	case []catalog.NoResultsResponse:
		if len(v) > 0 {
			return v[0], true
		}
	case []*catalog.NoResultsResponse:
		if len(v) > 0 && v[0] != nil {
			return *v[0], true
		}
	}
	return catalog.NoResultsResponse{}, false
}

// IsSeguroCatalog checks if the catalog name is an insurance catalog (case-insensitive prefix "seguros").
func IsSeguroCatalog(catalogName string) bool {
	return strings.HasPrefix(strings.ToLower(catalogName), "seguros")
}

// IsTiposDocumentosCatalog checks if the catalog name is exactly "TiposDocumentos" (case-sensitive).
func IsTiposDocumentosCatalog(catalogName string) bool {
	return catalogName == "TiposDocumentos"
}

// IsComunasCatalog checks if the catalog name is exactly "Comunas" (case-sensitive).
func IsComunasCatalog(catalogName string) bool {
	return catalogName == "Comunas"
}

// IsSeguroCatalog on Client checks if the catalog name is an insurance catalog.
func (c *Client) IsSeguroCatalog(catalogName string) bool {
	return IsSeguroCatalog(catalogName)
}

// IsTiposDocumentosCatalog on Client checks if the catalog name is exactly "TiposDocumentos".
func (c *Client) IsTiposDocumentosCatalog(catalogName string) bool {
	return IsTiposDocumentosCatalog(catalogName)
}

// IsComunasCatalog on Client checks if the catalog name is exactly "Comunas".
func (c *Client) IsComunasCatalog(catalogName string) bool {
	return IsComunasCatalog(catalogName)
}

// GetCatalog queries FinnFlow for the specified catalog name performing only 1 HTTP call.
// It routes to the appropriate catalog decoder based on catalog matching rules:
// - Seguros* (case-insensitive prefix): returns []catalog.SeguroItem or []catalog.NoResultsResponse
// - TiposDocumentos (case-sensitive exact): returns []catalog.TipoDocumentoItem or []catalog.NoResultsResponse
// - Comunas (case-sensitive exact): returns []catalog.ComunaItem or []catalog.NoResultsResponse
// - General catalogs: returns []catalog.CatalogItem or []catalog.NoResultsResponse
//
// Error handling:
// - 404 from FinnFlow: returns error translated to HTTP 402 (ErrCatalogNotFound).
// - Timeout or network error: returns []catalog.NoResultsResponse with HTTP status 200.
func (c *Client) GetCatalog(ctx context.Context, catalogName string) (any, int, error) {
	if IsSeguroCatalog(catalogName) {
		return c.GetSeguroItems(ctx, catalogName)
	}
	if IsTiposDocumentosCatalog(catalogName) {
		return c.GetTiposDocumentosItems(ctx, catalogName)
	}
	if IsComunasCatalog(catalogName) {
		return c.GetComunasItems(ctx, catalogName)
	}
	return c.GetCatalogItems(ctx, catalogName)
}

// GetCatalogItems retrieves standard catalog items (e.g. Destino, Objetivo, Regiones, etc.).
func (c *Client) GetCatalogItems(ctx context.Context, catalogName string) (any, int, error) {
	bodyBytes, statusCode, isFallback, err := c.executeSingleCall(ctx, catalogName)
	if isFallback {
		return FallbackNoResultsList(), http.StatusOK, nil
	}
	if err != nil {
		return nil, statusCode, err
	}

	if isNoResultsBody(bodyBytes) {
		var noResultsList []catalog.NoResultsResponse
		if err := json.Unmarshal(bodyBytes, &noResultsList); err == nil && len(noResultsList) > 0 {
			return noResultsList, http.StatusOK, nil
		}
		var singleNoResults catalog.NoResultsResponse
		if err := json.Unmarshal(bodyBytes, &singleNoResults); err == nil {
			return []catalog.NoResultsResponse{singleNoResults}, http.StatusOK, nil
		}
		return FallbackNoResultsList(), http.StatusOK, nil
	}

	var items []catalog.CatalogItem
	if err := json.Unmarshal(bodyBytes, &items); err != nil {
		return nil, http.StatusBadRequest, NewCatalogError("Error en la obtención de catálogos")
	}

	if len(items) == 0 {
		return FallbackNoResultsList(), http.StatusOK, nil
	}

	return items, http.StatusOK, nil
}

// GetSeguroItems retrieves insurance catalog items (e.g. SegurosIncendio, SegurosDesgravamen, SegurosCesantia).
func (c *Client) GetSeguroItems(ctx context.Context, catalogName string) (any, int, error) {
	bodyBytes, statusCode, isFallback, err := c.executeSingleCall(ctx, catalogName)
	if isFallback {
		return FallbackNoResultsList(), http.StatusOK, nil
	}
	if err != nil {
		return nil, statusCode, err
	}

	if isNoResultsBody(bodyBytes) {
		var noResultsList []catalog.NoResultsResponse
		if err := json.Unmarshal(bodyBytes, &noResultsList); err == nil && len(noResultsList) > 0 {
			return noResultsList, http.StatusOK, nil
		}
		var singleNoResults catalog.NoResultsResponse
		if err := json.Unmarshal(bodyBytes, &singleNoResults); err == nil {
			return []catalog.NoResultsResponse{singleNoResults}, http.StatusOK, nil
		}
		return FallbackNoResultsList(), http.StatusOK, nil
	}

	var items []catalog.SeguroItem
	if err := json.Unmarshal(bodyBytes, &items); err != nil {
		return nil, http.StatusBadRequest, NewCatalogError("Error en la obtención de catálogos")
	}

	if len(items) == 0 {
		return FallbackNoResultsList(), http.StatusOK, nil
	}

	return items, http.StatusOK, nil
}

// GetComunasItems retrieves comunas catalog items including region_id.
func (c *Client) GetComunasItems(ctx context.Context, catalogName string) (any, int, error) {
	bodyBytes, statusCode, isFallback, err := c.executeSingleCall(ctx, catalogName)
	if isFallback {
		return FallbackNoResultsList(), http.StatusOK, nil
	}
	if err != nil {
		return nil, statusCode, err
	}

	if isNoResultsBody(bodyBytes) {
		var noResultsList []catalog.NoResultsResponse
		if err := json.Unmarshal(bodyBytes, &noResultsList); err == nil && len(noResultsList) > 0 {
			return noResultsList, http.StatusOK, nil
		}
		var singleNoResults catalog.NoResultsResponse
		if err := json.Unmarshal(bodyBytes, &singleNoResults); err == nil {
			return []catalog.NoResultsResponse{singleNoResults}, http.StatusOK, nil
		}
		return FallbackNoResultsList(), http.StatusOK, nil
	}

	var items []catalog.ComunaItem
	if err := json.Unmarshal(bodyBytes, &items); err != nil {
		return nil, http.StatusBadRequest, NewCatalogError("Error en la obtención de catálogos")
	}

	if len(items) == 0 {
		return FallbackNoResultsList(), http.StatusOK, nil
	}

	return items, http.StatusOK, nil
}

// GetTiposDocumentosItems retrieves document type items including grupo_id.
func (c *Client) GetTiposDocumentosItems(ctx context.Context, catalogName string) (any, int, error) {
	bodyBytes, statusCode, isFallback, err := c.executeSingleCall(ctx, catalogName)
	if isFallback {
		return FallbackNoResultsList(), http.StatusOK, nil
	}
	if err != nil {
		return nil, statusCode, err
	}

	if isNoResultsBody(bodyBytes) {
		var noResultsList []catalog.NoResultsResponse
		if err := json.Unmarshal(bodyBytes, &noResultsList); err == nil && len(noResultsList) > 0 {
			return noResultsList, http.StatusOK, nil
		}
		var singleNoResults catalog.NoResultsResponse
		if err := json.Unmarshal(bodyBytes, &singleNoResults); err == nil {
			return []catalog.NoResultsResponse{singleNoResults}, http.StatusOK, nil
		}
		return FallbackNoResultsList(), http.StatusOK, nil
	}

	var items []catalog.TipoDocumentoItem
	if err := json.Unmarshal(bodyBytes, &items); err != nil {
		return nil, http.StatusBadRequest, NewCatalogError("Error en la obtención de catálogos")
	}

	if len(items) == 0 {
		return FallbackNoResultsList(), http.StatusOK, nil
	}

	return items, http.StatusOK, nil
}

// GetCatalogRaw executes the single HTTP call and returns the raw JSON bytes, status code, and translated error.
// On timeout or network error, it returns the serialized NoResultsResponse JSON with HTTP 200.
func (c *Client) GetCatalogRaw(ctx context.Context, catalogName string) ([]byte, int, error) {
	bodyBytes, statusCode, isFallback, err := c.executeSingleCall(ctx, catalogName)
	if isFallback {
		fallbackBytes, _ := json.Marshal(FallbackNoResultsList())
		return fallbackBytes, http.StatusOK, nil
	}
	if err != nil {
		return bodyBytes, statusCode, err
	}
	return bodyBytes, statusCode, nil
}
