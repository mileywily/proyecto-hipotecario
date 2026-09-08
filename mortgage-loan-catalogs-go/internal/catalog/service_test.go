package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// mockFinnflowClient implements FinnflowClient for testing.
type mockFinnflowClient struct {
	rawResponse []byte
	statusCode  int
	err         error
}

func (m *mockFinnflowClient) GetCatalogRaw(ctx context.Context, catalogName string) ([]byte, int, error) {
	return m.rawResponse, m.statusCode, m.err
}

func TestCatalogRoutingRules(t *testing.T) {
	testCases := []struct {
		catalogName  string
		expectedType CatalogType
		isSeguro     bool
		isTiposDoc   bool
		isComunas    bool
	}{
		// Seguros* (case-insensitive prefix match)
		{"SegurosIncendio", CatalogTypeSeguros, true, false, false},
		{"SegurosDesgravamen", CatalogTypeSeguros, true, false, false},
		{"SegurosCesantia", CatalogTypeSeguros, true, false, false},
		{"segurosincendio", CatalogTypeSeguros, true, false, false},
		{"SEGUROS_TOTAL", CatalogTypeSeguros, true, false, false},
		{"seguros", CatalogTypeSeguros, true, false, false},
		{"SEGUROS", CatalogTypeSeguros, true, false, false},
		{"seguro", CatalogTypeGeneric, false, false, false},   // Missing trailing 's'
		{"MiSeguro", CatalogTypeGeneric, false, false, false}, // Not a prefix

		// TiposDocumentos (case-sensitive exact match)
		{"TiposDocumentos", CatalogTypeTiposDocumentos, false, true, false},
		{"tiposdocumentos", CatalogTypeGeneric, false, false, false}, // Lowercase -> fallback
		{"TIPOSDOCUMENTOS", CatalogTypeGeneric, false, false, false},
		{"Tiposdocumentos", CatalogTypeGeneric, false, false, false},
		{"TiposDocumento", CatalogTypeGeneric, false, false, false},

		// Comunas (case-sensitive exact match)
		{"Comunas", CatalogTypeComunas, false, false, true},
		{"comunas", CatalogTypeGeneric, false, false, false}, // Lowercase -> fallback
		{"COMUNAS", CatalogTypeGeneric, false, false, false},
		{"Comuna", CatalogTypeGeneric, false, false, false},

		// Standard catalogs -> Generic fallback
		{"Destino", CatalogTypeGeneric, false, false, false},
		{"Objetivo", CatalogTypeGeneric, false, false, false},
		{"Regiones", CatalogTypeGeneric, false, false, false},
		{"Producto", CatalogTypeGeneric, false, false, false},
		{"GruposDocumentos", CatalogTypeGeneric, false, false, false},
		{"TipoParticipante", CatalogTypeGeneric, false, false, false},
		{"CampanasHipotecarias", CatalogTypeGeneric, false, false, false},
	}

	for _, tc := range testCases {
		t.Run(tc.catalogName, func(t *testing.T) {
			gotType := RouteCatalog(tc.catalogName)
			if gotType != tc.expectedType {
				t.Errorf("RouteCatalog(%q) = %s, want %s", tc.catalogName, gotType, tc.expectedType)
			}

			if got := IsSeguroCatalog(tc.catalogName); got != tc.isSeguro {
				t.Errorf("IsSeguroCatalog(%q) = %v, want %v", tc.catalogName, got, tc.isSeguro)
			}

			if got := IsTiposDocumentosCatalog(tc.catalogName); got != tc.isTiposDoc {
				t.Errorf("IsTiposDocumentosCatalog(%q) = %v, want %v", tc.catalogName, got, tc.isTiposDoc)
			}

			if got := IsComunasCatalog(tc.catalogName); got != tc.isComunas {
				t.Errorf("IsComunasCatalog(%q) = %v, want %v", tc.catalogName, got, tc.isComunas)
			}
		})
	}
}

func TestMapCatalogBodySeguros(t *testing.T) {
	payload := `[{"IdentificadorSeguro":"1100438-01-2023-000","Descripcion":"INCENDIO - Everest","Poliza":"100438-01-2023-000","NombreCompania":"Everest","CodigoCompania":39,"CodigoTipoSeguro":0,"CorrelativoPoliza":28,"Tasa":0.22553,"Factor":2.255E-4,"IndicadorPolizaIndividual":0,"PorValorCuota":0,"IndicadorPolizaExterna":0}]`

	// Test PascalCase "SegurosIncendio"
	res1, err := MapCatalogBody("SegurosIncendio", []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	items1, ok := res1.([]SeguroItem)
	if !ok || len(items1) != 1 {
		t.Fatalf("expected []SeguroItem with 1 item, got %T", res1)
	}
	if items1[0].IdentificadorSeguro != "1100438-01-2023-000" {
		t.Errorf("got %s, want 1100438-01-2023-000", items1[0].IdentificadorSeguro)
	}
	if items1[0].Factor == nil || items1[0].Factor.Float64() != 0.0002255 {
		t.Errorf("factor mismatch: got %v", items1[0].Factor)
	}

	// Test lowercase prefix "segurosincendio" (case-insensitive prefix rule)
	res2, err := MapCatalogBody("segurosincendio", []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	items2, ok := res2.([]SeguroItem)
	if !ok || len(items2) != 1 {
		t.Fatalf("expected []SeguroItem for 'segurosincendio', got %T", res2)
	}
	if items2[0].IdentificadorSeguro != "1100438-01-2023-000" {
		t.Errorf("got %s, want 1100438-01-2023-000", items2[0].IdentificadorSeguro)
	}
}

func TestMapCatalogBodyTiposDocumentosCaseSensitivity(t *testing.T) {
	docPayload := `[{"codigo_adm":1,"descripcion":"Cédula de Identidad","grupo_id":10}]`

	// 1. Exact case "TiposDocumentos" -> mapped to []TipoDocumentoItem (preserves grupo_id)
	resExact, err := MapCatalogBody("TiposDocumentos", []byte(docPayload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	docItems, ok := resExact.([]TipoDocumentoItem)
	if !ok || len(docItems) != 1 {
		t.Fatalf("expected []TipoDocumentoItem, got %T", resExact)
	}
	if docItems[0].GrupoId == nil || *docItems[0].GrupoId != 10 {
		t.Errorf("expected GrupoId 10, got %v", docItems[0].GrupoId)
	}
	if docItems[0].CodigoAdm == nil || *docItems[0].CodigoAdm != 1 {
		t.Errorf("expected CodigoAdm 1, got %v", docItems[0].CodigoAdm)
	}

	// 2. Lowercase "tiposdocumentos" -> falls through to generic []CatalogItem (drops grupo_id)
	resLower, err := MapCatalogBody("tiposdocumentos", []byte(docPayload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stdItems, ok := resLower.([]CatalogItem)
	if !ok || len(stdItems) != 1 {
		t.Fatalf("expected []CatalogItem for lowercase 'tiposdocumentos', got %T", resLower)
	}
	if stdItems[0].CodigoAdm != "1" || stdItems[0].Descripcion != "Cédula de Identidad" {
		t.Errorf("unexpected standard item: %+v", stdItems[0])
	}

	// Verify serialized JSON for the fallback does NOT contain "grupo_id"
	jsonBytes, err := json.Marshal(stdItems)
	if err != nil {
		t.Fatalf("failed to marshal fallback items: %v", err)
	}
	if strings.Contains(string(jsonBytes), "grupo_id") {
		t.Errorf("generic CatalogItem should not contain 'grupo_id', got %s", string(jsonBytes))
	}
}

func TestMapCatalogBodyComunasCaseSensitivity(t *testing.T) {
	comunaPayload := `[{"codigo_adm":"13101","descripcion":"Santiago","region_id":13}]`

	// 1. Exact case "Comunas" -> mapped to []ComunaItem (preserves region_id)
	resExact, err := MapCatalogBody("Comunas", []byte(comunaPayload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	comunaItems, ok := resExact.([]ComunaItem)
	if !ok || len(comunaItems) != 1 {
		t.Fatalf("expected []ComunaItem, got %T", resExact)
	}
	if comunaItems[0].RegionId == nil || *comunaItems[0].RegionId != 13 {
		t.Errorf("expected RegionId 13, got %v", comunaItems[0].RegionId)
	}

	// 2. Lowercase "comunas" -> falls through to generic []CatalogItem (drops region_id)
	resLower, err := MapCatalogBody("comunas", []byte(comunaPayload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stdItems, ok := resLower.([]CatalogItem)
	if !ok || len(stdItems) != 1 {
		t.Fatalf("expected []CatalogItem for lowercase 'comunas', got %T", resLower)
	}
	if stdItems[0].CodigoAdm != "13101" || stdItems[0].Descripcion != "Santiago" {
		t.Errorf("unexpected standard item: %+v", stdItems[0])
	}

	// Verify serialized JSON for the fallback does NOT contain "region_id"
	jsonBytes, err := json.Marshal(stdItems)
	if err != nil {
		t.Fatalf("failed to marshal fallback items: %v", err)
	}
	if strings.Contains(string(jsonBytes), "region_id") {
		t.Errorf("generic CatalogItem should not contain 'region_id', got %s", string(jsonBytes))
	}
}

func TestMapCatalogBodyGenericCatalogs(t *testing.T) {
	payload := `[{"codigo_adm":"1","descripcion":"Vivienda Principal"}]`

	res, err := MapCatalogBody("Destino", []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	items, ok := res.([]CatalogItem)
	if !ok || len(items) != 1 {
		t.Fatalf("expected []CatalogItem, got %T", res)
	}
	if items[0].CodigoAdm != "1" || items[0].Descripcion != "Vivienda Principal" {
		t.Errorf("unexpected item: %+v", items[0])
	}
}

func TestMapCatalogBodySinResultadosAndEmpty(t *testing.T) {
	noResultsPayload := `[{"codRespuesta":3,"Mensaje":"Sin resultados.","Excepcion":"Ninguna"}]`

	catalogs := []string{"SegurosIncendio", "TiposDocumentos", "Comunas", "Destino", "tiposdocumentos"}
	for _, cat := range catalogs {
		t.Run(cat, func(t *testing.T) {
			res, err := MapCatalogBody(cat, []byte(noResultsPayload))
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", cat, err)
			}
			noRes, ok := res.([]NoResultsResponse)
			if !ok || len(noRes) != 1 {
				t.Fatalf("expected []NoResultsResponse for %s, got %T", cat, res)
			}
			if noRes[0].CodRespuesta != 3 || noRes[0].Mensaje != "Sin resultados." {
				t.Errorf("unexpected NoResultsResponse: %+v", noRes[0])
			}
		})
	}

	// Test empty array payload
	emptyArrayPayload := `[]`
	for _, cat := range catalogs {
		t.Run(cat+"_empty", func(t *testing.T) {
			res, err := MapCatalogBody(cat, []byte(emptyArrayPayload))
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", cat, err)
			}
			noRes, ok := res.([]NoResultsResponse)
			if !ok || len(noRes) != 1 {
				t.Fatalf("expected fallback []NoResultsResponse for empty array, got %T", res)
			}
		})
	}
}

func TestServiceWithClient(t *testing.T) {
	mockClient := &mockFinnflowClient{
		rawResponse: []byte(`[{"codigo_adm":"1","descripcion":"Vivienda Principal"}]`),
		statusCode:  http.StatusOK,
		err:         nil,
	}

	svc := NewService(mockClient)

	// Test GetCatalog
	data, code, err := svc.GetCatalog(context.Background(), "Destino")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != http.StatusOK {
		t.Errorf("got code %d, want 200", code)
	}
	items, ok := data.([]CatalogItem)
	if !ok || len(items) != 1 || items[0].CodigoAdm != "1" {
		t.Fatalf("unexpected items: %+v", data)
	}

	// Test error propagation
	mockErrClient := &mockFinnflowClient{
		rawResponse: nil,
		statusCode:  http.StatusPaymentRequired,
		err:         ErrCatalogNotFound,
	}
	svcErr := NewService(mockErrClient)
	_, errCode, err := svcErr.GetCatalog(context.Background(), "Inexistente")
	if !errors.Is(err, ErrCatalogNotFound) {
		t.Errorf("expected ErrCatalogNotFound, got %v", err)
	}
	if errCode != http.StatusPaymentRequired {
		t.Errorf("got code %d, want 402", errCode)
	}

	// Test service helpers match package-level functions
	if !svc.IsSeguroCatalog("segurosdesgravamen") {
		t.Error("svc.IsSeguroCatalog failed")
	}
	if !svc.IsTiposDocumentosCatalog("TiposDocumentos") {
		t.Error("svc.IsTiposDocumentosCatalog failed")
	}
	if !svc.IsComunasCatalog("Comunas") {
		t.Error("svc.IsComunasCatalog failed")
	}
	if svc.RouteCatalog("tiposdocumentos") != CatalogTypeGeneric {
		t.Error("svc.RouteCatalog lowercase tiposdocumentos should be generic")
	}
}
