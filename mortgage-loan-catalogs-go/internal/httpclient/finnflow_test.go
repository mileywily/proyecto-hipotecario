package httpclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"mortgage-loan-catalogs-go/internal/catalog"
)

func TestFinnflowClientSingleCallOptimization(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)

		// Verify request endpoint
		expectedPath := "/api/catalogo_detail/Destino"
		if r.URL.Path != expectedPath {
			t.Errorf("got path %s, want %s", r.URL.Path, expectedPath)
		}

		// Verify Basic Auth
		authHeader := r.Header.Get("Authorization")
		expectedCreds := base64.StdEncoding.EncodeToString([]byte("testkey:testsecret"))
		if authHeader != "Basic "+expectedCreds {
			t.Errorf("got auth %s, want Basic %s", authHeader, expectedCreds)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"codigo_adm":"1","descripcion":"Vivienda Principal"}]`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:  server.URL,
		Username: "testkey",
		Password: "testsecret",
	})

	res, statusCode, err := client.GetCatalog(context.Background(), "Destino")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if statusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", statusCode, http.StatusOK)
	}

	// Verify ONLY 1 HTTP call was made
	if count := atomic.LoadInt32(&callCount); count != 1 {
		t.Fatalf("expected exactly 1 outbound call, got %d", count)
	}

	items, ok := res.([]catalog.CatalogItem)
	if !ok {
		t.Fatalf("expected []catalog.CatalogItem, got %T", res)
	}
	if len(items) != 1 || items[0].CodigoAdm != "1" || items[0].Descripcion != "Vivienda Principal" {
		t.Errorf("unexpected items: %+v", items)
	}
}

func TestFinnflowClient404TranslationTo402(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Not found"}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
	})

	_, statusCode, err := client.GetCatalog(context.Background(), "CatalogoInexistente")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if statusCode != http.StatusPaymentRequired {
		t.Errorf("got status %d, want 402 (Payment Required)", statusCode)
	}

	if !IsCatalogNotFound(err) {
		t.Errorf("IsCatalogNotFound(err) should be true, got false for: %v", err)
	}

	if err.Error() != "Catálogo no encontrado" {
		t.Errorf("got error message %q, want %q", err.Error(), "Catálogo no encontrado")
	}

	if sc := StatusCodeFromError(err, 500); sc != 402 {
		t.Errorf("StatusCodeFromError got %d, want 402", sc)
	}
}

func TestFinnflowClientTimeoutReturns200WithNoResultsResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the client's timeout
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		Timeout: 20 * time.Millisecond,
	})

	res, statusCode, err := client.GetCatalog(context.Background(), "Destino")
	if err != nil {
		t.Fatalf("expected nil error on timeout, got %v", err)
	}

	if statusCode != http.StatusOK {
		t.Errorf("got status %d, want %d (OK)", statusCode, http.StatusOK)
	}

	noResultsList, ok := res.([]catalog.NoResultsResponse)
	if !ok {
		t.Fatalf("expected []catalog.NoResultsResponse, got %T", res)
	}

	if len(noResultsList) != 1 {
		t.Fatalf("expected 1 item in fallback list, got %d", len(noResultsList))
	}

	expected := catalog.NewNoResultsResponse()
	if noResultsList[0] != expected {
		t.Errorf("got %+v, want %+v", noResultsList[0], expected)
	}

	// Verify JSON byte-for-byte serialization matches rule 0: [{"codRespuesta": 3, "Mensaje": "Sin resultados.", "Excepcion": "Ninguna"}]
	jsonData, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshaling error: %v", err)
	}
	expectedJSON := `[{"codRespuesta":3,"Mensaje":"Sin resultados.","Excepcion":"Ninguna"}]`
	if string(jsonData) != expectedJSON {
		t.Errorf("got JSON %s, want %s", string(jsonData), expectedJSON)
	}

	// Verify helper AsNoResultsResponse
	if nr, found := AsNoResultsResponse(res); !found || nr.CodRespuesta != 3 {
		t.Errorf("AsNoResultsResponse failed: %+v, found=%v", nr, found)
	}
}

func TestFinnflowClientNetworkErrorReturns200WithNoResultsResponse(t *testing.T) {
	// Find an unused closed port to guarantee connection refused network error
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	closedAddr := listener.Addr().String()
	_ = listener.Close()

	client := NewClient(Config{
		BaseURL: "http://" + closedAddr,
		Timeout: 500 * time.Millisecond,
	})

	res, statusCode, err := client.GetCatalog(context.Background(), "Destino")
	if err != nil {
		t.Fatalf("expected nil error on network error, got %v", err)
	}

	if statusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", statusCode, http.StatusOK)
	}

	noResultsList, ok := res.([]catalog.NoResultsResponse)
	if !ok || len(noResultsList) != 1 {
		t.Fatalf("expected fallback []catalog.NoResultsResponse of length 1, got %T (%+v)", res, res)
	}

	if noResultsList[0].CodRespuesta != 3 || noResultsList[0].Mensaje != "Sin resultados." {
		t.Errorf("unexpected fallback response: %+v", noResultsList[0])
	}
}

func TestFinnflowClientSinResultadosBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"codRespuesta":3,"Mensaje":"Sin resultados.","Excepcion":"Ninguna"}]`))
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL})

	res, statusCode, err := client.GetCatalog(context.Background(), "Destino")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if statusCode != http.StatusOK {
		t.Errorf("got status %d, want 200", statusCode)
	}

	noResults, ok := res.([]catalog.NoResultsResponse)
	if !ok || len(noResults) != 1 || noResults[0].CodRespuesta != 3 {
		t.Fatalf("expected parsed NoResultsResponse, got %+v", res)
	}
}

func TestFinnflowClientSegurosAsymmetry(t *testing.T) {
	seguroPayload := `[{"IdentificadorSeguro":"1100438-01-2023-000","Descripcion":"INCENDIO - Everest","Poliza":"100438-01-2023-000","NombreCompania":"Everest","CodigoCompania":39,"CodigoTipoSeguro":0,"CorrelativoPoliza":28,"Tasa":0.22553,"Factor":2.255E-4,"IndicadorPolizaIndividual":0,"PorValorCuota":0,"IndicadorPolizaExterna":0}]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(seguroPayload))
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL})

	// Case-insensitive test for "segurosincendio"
	res, statusCode, err := client.GetCatalog(context.Background(), "segurosincendio")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if statusCode != http.StatusOK {
		t.Errorf("got status %d, want 200", statusCode)
	}

	seguros, ok := res.([]catalog.SeguroItem)
	if !ok || len(seguros) != 1 {
		t.Fatalf("expected []catalog.SeguroItem, got %T", res)
	}

	if seguros[0].IdentificadorSeguro != "1100438-01-2023-000" {
		t.Errorf("got %s, want 1100438-01-2023-000", seguros[0].IdentificadorSeguro)
	}
	if seguros[0].Factor == nil || seguros[0].Factor.Float64() != 0.0002255 {
		t.Errorf("factor mismatch: got %v", seguros[0].Factor)
	}
}

func TestFinnflowClientTiposDocumentosCaseSensitivity(t *testing.T) {
	docPayload := `[{"codigo_adm":1,"descripcion":"Cédula","grupo_id":10}]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(docPayload))
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL})

	// Exact case "TiposDocumentos" -> preserves grupo_id in TipoDocumentoItem
	resExact, _, err := client.GetCatalog(context.Background(), "TiposDocumentos")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	docItems, ok := resExact.([]catalog.TipoDocumentoItem)
	if !ok || len(docItems) != 1 {
		t.Fatalf("expected []catalog.TipoDocumentoItem, got %T", resExact)
	}
	if docItems[0].GrupoId == nil || *docItems[0].GrupoId != 10 {
		t.Errorf("expected GrupoId 10, got %v", docItems[0].GrupoId)
	}

	// Lowercase "tiposdocumentos" -> falls through to standard CatalogItem (loses grupo_id per rule 0)
	// Upstream returns `[{"codigo_adm": "1", "descripcion": "Cédula", "grupo_id": 10}]`
	serverStandard := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"codigo_adm":"1","descripcion":"Cédula","grupo_id":10}]`))
	}))
	defer serverStandard.Close()

	clientStandard := NewClient(Config{BaseURL: serverStandard.URL})
	resLower, _, err := clientStandard.GetCatalog(context.Background(), "tiposdocumentos")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stdItems, ok := resLower.([]catalog.CatalogItem)
	if !ok || len(stdItems) != 1 {
		t.Fatalf("expected []catalog.CatalogItem for lowercase tiposdocumentos, got %T", resLower)
	}
	if stdItems[0].CodigoAdm != "1" || stdItems[0].Descripcion != "Cédula" {
		t.Errorf("unexpected standard item: %+v", stdItems[0])
	}
}

func TestFinnflowClientComunasCaseSensitivity(t *testing.T) {
	comunaPayload := `[{"codigo_adm":"13101","descripcion":"Santiago","region_id":13}]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(comunaPayload))
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL})

	// Exact case "Comunas" -> preserves region_id in ComunaItem
	resExact, _, err := client.GetCatalog(context.Background(), "Comunas")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	comunaItems, ok := resExact.([]catalog.ComunaItem)
	if !ok || len(comunaItems) != 1 {
		t.Fatalf("expected []catalog.ComunaItem, got %T", resExact)
	}
	if comunaItems[0].RegionId == nil || *comunaItems[0].RegionId != 13 {
		t.Errorf("expected RegionId 13, got %v", comunaItems[0].RegionId)
	}
}

func TestFinnflowClient401Translation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"Unauthorized"}`))
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL})

	_, statusCode, err := client.GetCatalog(context.Background(), "Destino")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if statusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want 401", statusCode)
	}
	if sc := StatusCodeFromError(err, 500); sc != 401 {
		t.Errorf("StatusCodeFromError got %d, want 401", sc)
	}
}

func TestFinnflowClient400Translation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors_detail":"Bad Request"}`))
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL})

	_, statusCode, err := client.GetCatalog(context.Background(), "Destino")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if statusCode != http.StatusBadRequest {
		t.Errorf("got status %d, want 400", statusCode)
	}
	if err.Error() != "Error en la obtención de catálogos" {
		t.Errorf("got error %q, want Error en la obtención de catálogos", err.Error())
	}
}

func TestFinnflowClientGetCatalogRaw(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"codigo_adm":"1","descripcion":"Vivienda Principal"}]`))
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL})

	raw, statusCode, err := client.GetCatalogRaw(context.Background(), "Destino")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if statusCode != http.StatusOK {
		t.Errorf("got status %d, want 200", statusCode)
	}
	expected := `[{"codigo_adm":"1","descripcion":"Vivienda Principal"}]`
	if string(raw) != expected {
		t.Errorf("got raw %s, want %s", string(raw), expected)
	}
}
