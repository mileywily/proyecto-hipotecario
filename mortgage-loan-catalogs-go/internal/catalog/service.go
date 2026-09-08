package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// CatalogType represents the category of a catalog.
type CatalogType string

const (
	CatalogTypeSeguros         CatalogType = "SEGUROS"
	CatalogTypeTiposDocumentos CatalogType = "TIPOS_DOCUMENTOS"
	CatalogTypeComunas         CatalogType = "COMUNAS"
	CatalogTypeGeneric         CatalogType = "GENERIC"
)

// Sentinel errors matching the domain and Java exception contracts.
var (
	ErrCatalogError    = errors.New("Error en la obtención de catálogos")
	ErrCatalogNotFound = errors.New("Catálogo no encontrado")
	ErrTokenNotValid   = errors.New("Token no válido")
)

// IsSeguroCatalog checks if the catalog name is an insurance catalog.
// Matching rule: case-insensitive prefix "seguros".
func IsSeguroCatalog(catalogName string) bool {
	return strings.HasPrefix(strings.ToLower(catalogName), "seguros")
}

// IsTiposDocumentosCatalog checks if the catalog name is exactly "TiposDocumentos".
// Matching rule: exact case-sensitive comparison.
func IsTiposDocumentosCatalog(catalogName string) bool {
	return catalogName == "TiposDocumentos"
}

// IsComunasCatalog checks if the catalog name is exactly "Comunas".
// Matching rule: exact case-sensitive comparison.
func IsComunasCatalog(catalogName string) bool {
	return catalogName == "Comunas"
}

// RouteCatalog resolves the catalog category using the exact rules:
// - Seguros* via case-insensitive prefix match
// - TiposDocumentos via exact case-sensitive match
// - Comunas via exact case-sensitive match
// - Fallback to generic CatalogItem[] (e.g. tiposdocumentos in lowercase falls back to generic)
func RouteCatalog(catalogName string) CatalogType {
	if IsSeguroCatalog(catalogName) {
		return CatalogTypeSeguros
	}
	if IsTiposDocumentosCatalog(catalogName) {
		return CatalogTypeTiposDocumentos
	}
	if IsComunasCatalog(catalogName) {
		return CatalogTypeComunas
	}
	return CatalogTypeGeneric
}

// FinnflowClient defines the interface for communicating with the FinnFlow upstream service.
type FinnflowClient interface {
	GetCatalogRaw(ctx context.Context, catalogName string) ([]byte, int, error)
}

// FinnflowRawClient is an alias for FinnflowClient.
type FinnflowRawClient = FinnflowClient

// Service provides catalog routing, fetching, and response body mapping.
type Service struct {
	client FinnflowClient
}

// NewService creates a new Service instance.
func NewService(client FinnflowClient) *Service {
	return &Service{client: client}
}

// IsSeguroCatalog checks if the catalog name is an insurance catalog.
func (s *Service) IsSeguroCatalog(catalogName string) bool {
	return IsSeguroCatalog(catalogName)
}

// IsTiposDocumentosCatalog checks if the catalog name is exactly "TiposDocumentos".
func (s *Service) IsTiposDocumentosCatalog(catalogName string) bool {
	return IsTiposDocumentosCatalog(catalogName)
}

// IsComunasCatalog checks if the catalog name is exactly "Comunas".
func (s *Service) IsComunasCatalog(catalogName string) bool {
	return IsComunasCatalog(catalogName)
}

// RouteCatalog determines the catalog type.
func (s *Service) RouteCatalog(catalogName string) CatalogType {
	return RouteCatalog(catalogName)
}

// MapCatalogBody routes the catalog name and maps the FinnFlow response body to the corresponding struct:
// - Seguros* (case-insensitive prefix) -> []SeguroItem or []NoResultsResponse
// - TiposDocumentos (case-sensitive exact) -> []TipoDocumentoItem or []NoResultsResponse
// - Comunas (case-sensitive exact) -> []ComunaItem or []NoResultsResponse
// - Generic fallback (Destino, Objetivo, Regiones, and lowercase 'tiposdocumentos'/'comunas') -> []CatalogItem or []NoResultsResponse
func (s *Service) MapCatalogBody(catalogName string, body []byte) (any, error) {
	return MapCatalogBody(catalogName, body)
}

// MapBody is an alias for MapCatalogBody.
func (s *Service) MapBody(catalogName string, body []byte) (any, error) {
	return MapCatalogBody(catalogName, body)
}

// GetCatalog fetches and maps the catalog using the configured FinnFlow client.
func (s *Service) GetCatalog(ctx context.Context, catalogName string) (any, int, error) {
	if s.client == nil {
		return nil, http.StatusInternalServerError, errors.New("finnflow client is nil")
	}

	bodyBytes, statusCode, err := s.client.GetCatalogRaw(ctx, catalogName)
	if err != nil {
		return nil, statusCode, err
	}

	mapped, err := MapCatalogBody(catalogName, bodyBytes)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	return mapped, statusCode, nil
}

// GetSeguroItems retrieves insurance catalog items (e.g. SegurosIncendio, SegurosDesgravamen).
func (s *Service) GetSeguroItems(ctx context.Context, catalogName string) (any, int, error) {
	if s.client == nil {
		return nil, http.StatusInternalServerError, errors.New("finnflow client is nil")
	}
	bodyBytes, statusCode, err := s.client.GetCatalogRaw(ctx, catalogName)
	if err != nil {
		return nil, statusCode, err
	}
	mapped, err := MapSeguroItems(bodyBytes)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	return mapped, statusCode, nil
}

// GetTiposDocumentosItems retrieves document type items preserving grupo_id.
func (s *Service) GetTiposDocumentosItems(ctx context.Context, catalogName string) (any, int, error) {
	if s.client == nil {
		return nil, http.StatusInternalServerError, errors.New("finnflow client is nil")
	}
	bodyBytes, statusCode, err := s.client.GetCatalogRaw(ctx, catalogName)
	if err != nil {
		return nil, statusCode, err
	}
	mapped, err := MapTipoDocumentoItems(bodyBytes)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	return mapped, statusCode, nil
}

// GetComunasItems retrieves comunas catalog items preserving region_id.
func (s *Service) GetComunasItems(ctx context.Context, catalogName string) (any, int, error) {
	if s.client == nil {
		return nil, http.StatusInternalServerError, errors.New("finnflow client is nil")
	}
	bodyBytes, statusCode, err := s.client.GetCatalogRaw(ctx, catalogName)
	if err != nil {
		return nil, statusCode, err
	}
	mapped, err := MapComunaItems(bodyBytes)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	return mapped, statusCode, nil
}

// GetCatalogItems retrieves standard catalog items (e.g. Destino, Objetivo, Regiones, or generic fallback).
func (s *Service) GetCatalogItems(ctx context.Context, catalogName string) (any, int, error) {
	if s.client == nil {
		return nil, http.StatusInternalServerError, errors.New("finnflow client is nil")
	}
	bodyBytes, statusCode, err := s.client.GetCatalogRaw(ctx, catalogName)
	if err != nil {
		return nil, statusCode, err
	}
	mapped, err := MapCatalogItems(bodyBytes)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	return mapped, statusCode, nil
}

// MapCatalogBody routes the catalog name and maps the FinnFlow response body to the corresponding struct.
func MapCatalogBody(catalogName string, body []byte) (any, error) {
	switch RouteCatalog(catalogName) {
	case CatalogTypeSeguros:
		return MapSeguroItems(body)
	case CatalogTypeTiposDocumentos:
		return MapTipoDocumentoItems(body)
	case CatalogTypeComunas:
		return MapComunaItems(body)
	default:
		return MapCatalogItems(body)
	}
}

// MapBody is an alias for MapCatalogBody.
func MapBody(catalogName string, body []byte) (any, error) {
	return MapCatalogBody(catalogName, body)
}

// IsNoResultsBody checks if the body matches the legacy upstream "Sin resultados" format or is empty/null.
func IsNoResultsBody(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("[]")) || bytes.Equal(trimmed, []byte("null")) {
		return true
	}
	s := string(trimmed)
	return strings.Contains(s, "codRespuesta") && strings.Contains(s, "Sin resultados")
}

// ParseNoResults extracts a slice of NoResultsResponse from the body or returns a default single-item slice.
func ParseNoResults(body []byte) []NoResultsResponse {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) > 0 {
		var list []NoResultsResponse
		if err := json.Unmarshal(trimmed, &list); err == nil && len(list) > 0 {
			return list
		}
		var single NoResultsResponse
		if err := json.Unmarshal(trimmed, &single); err == nil && single.CodRespuesta != 0 {
			return []NoResultsResponse{single}
		}
	}
	return []NoResultsResponse{NewNoResultsResponse()}
}

// MapSeguroItems maps the JSON body into []SeguroItem or []NoResultsResponse.
func MapSeguroItems(body []byte) (any, error) {
	if IsNoResultsBody(body) {
		return ParseNoResults(body), nil
	}

	var items []SeguroItem
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, ErrCatalogError
	}

	if len(items) == 0 {
		return []NoResultsResponse{NewNoResultsResponse()}, nil
	}

	return items, nil
}

// MapTipoDocumentoItems maps the JSON body into []TipoDocumentoItem or []NoResultsResponse.
func MapTipoDocumentoItems(body []byte) (any, error) {
	if IsNoResultsBody(body) {
		return ParseNoResults(body), nil
	}

	var items []TipoDocumentoItem
	if err := json.Unmarshal(body, &items); err != nil {
		// Fallback for flexible decoding if codigo_adm was provided as string or number
		var rawItems []rawTipoDocItem
		if errRaw := json.Unmarshal(body, &rawItems); errRaw == nil && len(rawItems) > 0 {
			items = make([]TipoDocumentoItem, len(rawItems))
			for i, r := range rawItems {
				items[i] = r.toTipoDocumentoItem()
			}
		} else {
			return nil, ErrCatalogError
		}
	}

	if len(items) == 0 {
		return []NoResultsResponse{NewNoResultsResponse()}, nil
	}

	return items, nil
}

// MapComunaItems maps the JSON body into []ComunaItem or []NoResultsResponse.
func MapComunaItems(body []byte) (any, error) {
	if IsNoResultsBody(body) {
		return ParseNoResults(body), nil
	}

	var items []ComunaItem
	if err := json.Unmarshal(body, &items); err != nil {
		var rawItems []rawComunaItem
		if errRaw := json.Unmarshal(body, &rawItems); errRaw == nil && len(rawItems) > 0 {
			items = make([]ComunaItem, len(rawItems))
			for i, r := range rawItems {
				items[i] = r.toComunaItem()
			}
		} else {
			return nil, ErrCatalogError
		}
	}

	if len(items) == 0 {
		return []NoResultsResponse{NewNoResultsResponse()}, nil
	}

	return items, nil
}

// MapCatalogItems maps the JSON body into generic []CatalogItem or []NoResultsResponse.
// Note: When lowercase 'tiposdocumentos' or 'comunas' falls back here, fields like grupo_id and region_id
// are omitted, matching the Java CatalogItem[] fallback contract.
func MapCatalogItems(body []byte) (any, error) {
	if IsNoResultsBody(body) {
		return ParseNoResults(body), nil
	}

	var items []CatalogItem
	if err := json.Unmarshal(body, &items); err != nil {
		var rawItems []rawCatalogItem
		if errRaw := json.Unmarshal(body, &rawItems); errRaw == nil && len(rawItems) > 0 {
			items = make([]CatalogItem, len(rawItems))
			for i, r := range rawItems {
				items[i] = CatalogItem{
					CodigoAdm:   rawToString(r.CodigoAdm),
					Descripcion: r.Descripcion,
				}
			}
		} else {
			return nil, ErrCatalogError
		}
	}

	if len(items) == 0 {
		return []NoResultsResponse{NewNoResultsResponse()}, nil
	}

	return items, nil
}

// Internal raw structs for tolerant parsing between upstream string/numeric representations.

type rawCatalogItem struct {
	CodigoAdm   json.RawMessage `json:"codigo_adm"`
	Descripcion string          `json:"descripcion"`
}

type rawComunaItem struct {
	CodigoAdm   json.RawMessage `json:"codigo_adm"`
	Descripcion string          `json:"descripcion"`
	RegionId    *int            `json:"region_id"`
}

func (r rawComunaItem) toComunaItem() ComunaItem {
	return ComunaItem{
		CodigoAdm:   rawToString(r.CodigoAdm),
		Descripcion: r.Descripcion,
		RegionId:    r.RegionId,
	}
}

type rawTipoDocItem struct {
	CodigoAdm   json.RawMessage `json:"codigo_adm"`
	Descripcion string          `json:"descripcion"`
	GrupoId     *int            `json:"grupo_id"`
}

func (r rawTipoDocItem) toTipoDocumentoItem() TipoDocumentoItem {
	var cod *int
	if len(r.CodigoAdm) > 0 {
		var n int
		if err := json.Unmarshal(r.CodigoAdm, &n); err == nil {
			cod = &n
		} else {
			var s string
			if err := json.Unmarshal(r.CodigoAdm, &s); err == nil {
				if parsed, err := strconv.Atoi(s); err == nil {
					cod = &parsed
				}
			}
		}
	}
	return TipoDocumentoItem{
		CodigoAdm:   cod,
		Descripcion: r.Descripcion,
		GrupoId:     r.GrupoId,
	}
}

func rawToString(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	var s string
	if err := json.Unmarshal(trimmed, &s); err == nil {
		return s
	}
	return string(trimmed)
}
