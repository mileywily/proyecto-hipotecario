package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mortgage-loan-catalogs-go/internal/catalog"
	"mortgage-loan-catalogs-go/internal/httpclient"
)

func TestHTTPServerEndpoints(t *testing.T) {
	// Mock FinnFlow upstream
	mockFinnFlow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/catalogo_detail/Destino":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"codigo_adm":"1","descripcion":"Vivienda Principal"}]`))

		case "/api/catalogo_detail/SegurosIncendio":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"IdentificadorSeguro":"1100438-01-2023-000","Descripcion":"INCENDIO - Everest","Poliza":"100438-01-2023-000","NombreCompania":"Everest","CodigoCompania":39,"CodigoTipoSeguro":0,"CorrelativoPoliza":28,"Tasa":0.22553,"Factor":2.255E-4,"IndicadorPolizaIndividual":0,"PorValorCuota":0,"IndicadorPolizaExterna":0}]`))

		case "/api/catalogo_detail/TiposDocumentos":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"codigo_adm":1,"descripcion":"Cédula de Identidad","grupo_id":10}]`))

		case "/api/catalogo_detail/tiposdocumentos":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"codigo_adm":1,"descripcion":"Cédula de Identidad","grupo_id":10}]`))

		case "/api/catalogo_detail/Comunas":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"codigo_adm":"13101","descripcion":"Santiago","region_id":13}]`))

		case "/api/catalogo_detail/comunas":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"codigo_adm":"13101","descripcion":"Santiago","region_id":13}]`))

		case "/api/catalogo_detail/Inexistente":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"detail":"Not Found"}`))

		case "/api/catalogo_detail/TokenInvalido":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"detail":"Token is invalid"}`))

		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer mockFinnFlow.Close()

	client := httpclient.NewClient(httpclient.Config{BaseURL: mockFinnFlow.URL})
	svc := catalog.NewService(client)
	router := SetupRouter(svc)

	server := httptest.NewServer(router)
	defer server.Close()

	// 1. Test GET /health
	t.Run("GET /health", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/health")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != `{"status":"UP"}` {
			t.Errorf("got %s, want %s", string(body), `{"status":"UP"}`)
		}
	})

	// 2. Test GET /ready
	t.Run("GET /ready", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/ready")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != `{"status":"UP"}` {
			t.Errorf("got %s, want %s", string(body), `{"status":"UP"}`)
		}
	})

	// 2b. Test GET /api-docs (OpenAPI specification)
	t.Run("GET /api-docs", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api-docs")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "yaml") {
			t.Errorf("expected yaml content-type, got %s", contentType)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Mortgage Loan Catalogs API") {
			t.Errorf("api-docs did not contain expected title")
		}
	})

	// 2c. Test GET /swagger-ui.html
	t.Run("GET /swagger-ui.html", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/swagger-ui.html")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "swagger-ui") {
			t.Errorf("swagger-ui.html did not contain swagger-ui DOM element")
		}
	})

	// 3. Test POST /v1/bfcl/mortgage-loan/catalogs/Destino
	t.Run("POST /v1/bfcl/mortgage-loan/catalogs/Destino", func(t *testing.T) {
		resp, err := http.Post(server.URL+"/v1/bfcl/mortgage-loan/catalogs/Destino", "application/json", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		expected := `[{"codigo_adm":"1","descripcion":"Vivienda Principal"}]`
		if string(body) != expected {
			t.Errorf("got %s, want %s", string(body), expected)
		}
	})

	// 4. Test POST /v1/bfcl/mortgage-loan/catalogs/SegurosIncendio
	t.Run("POST /v1/bfcl/mortgage-loan/catalogs/SegurosIncendio", func(t *testing.T) {
		resp, err := http.Post(server.URL+"/v1/bfcl/mortgage-loan/catalogs/SegurosIncendio", "application/json", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		expected := `[{"IdentificadorSeguro":"1100438-01-2023-000","Descripcion":"INCENDIO - Everest","Poliza":"100438-01-2023-000","NombreCompania":"Everest","CodigoCompania":39,"CodigoTipoSeguro":0,"CorrelativoPoliza":28,"Tasa":0.22553,"Factor":2.255E-4,"IndicadorPolizaIndividual":0,"PorValorCuota":0,"IndicadorPolizaExterna":0}]`
		if string(body) != expected {
			t.Errorf("got %s, want %s", string(body), expected)
		}
	})

	// 5. Test POST /v1/bfcl/mortgage-loan/catalogs/TiposDocumentos (preserves grupo_id)
	t.Run("POST /v1/bfcl/mortgage-loan/catalogs/TiposDocumentos", func(t *testing.T) {
		resp, err := http.Post(server.URL+"/v1/bfcl/mortgage-loan/catalogs/TiposDocumentos", "application/json", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		expected := `[{"codigo_adm":1,"descripcion":"Cédula de Identidad","grupo_id":10}]`
		if string(body) != expected {
			t.Errorf("got %s, want %s", string(body), expected)
		}
	})

	// 6. Test POST /v1/bfcl/mortgage-loan/catalogs/tiposdocumentos (lowercase falls back to generic, drops grupo_id)
	t.Run("POST /v1/bfcl/mortgage-loan/catalogs/tiposdocumentos", func(t *testing.T) {
		resp, err := http.Post(server.URL+"/v1/bfcl/mortgage-loan/catalogs/tiposdocumentos", "application/json", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		expected := `[{"codigo_adm":"1","descripcion":"Cédula de Identidad"}]`
		if string(body) != expected {
			t.Errorf("got %s, want %s", string(body), expected)
		}
		if strings.Contains(string(body), "grupo_id") {
			t.Errorf("should not contain grupo_id in fallback, got: %s", string(body))
		}
	})

	// 7. Test POST /v1/bfcl/mortgage-loan/catalogs/Comunas (preserves region_id)
	t.Run("POST /v1/bfcl/mortgage-loan/catalogs/Comunas", func(t *testing.T) {
		resp, err := http.Post(server.URL+"/v1/bfcl/mortgage-loan/catalogs/Comunas", "application/json", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		expected := `[{"codigo_adm":"13101","descripcion":"Santiago","region_id":13}]`
		if string(body) != expected {
			t.Errorf("got %s, want %s", string(body), expected)
		}
	})

	// 8. Test POST /v1/bfcl/mortgage-loan/catalogs/comunas (lowercase falls back to generic, drops region_id)
	t.Run("POST /v1/bfcl/mortgage-loan/catalogs/comunas", func(t *testing.T) {
		resp, err := http.Post(server.URL+"/v1/bfcl/mortgage-loan/catalogs/comunas", "application/json", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		expected := `[{"codigo_adm":"13101","descripcion":"Santiago"}]`
		if string(body) != expected {
			t.Errorf("got %s, want %s", string(body), expected)
		}
		if strings.Contains(string(body), "region_id") {
			t.Errorf("should not contain region_id in fallback, got: %s", string(body))
		}
	})

	// 9. Test POST /v1/bfcl/mortgage-loan/catalogs/Inexistente (402 Catálogo no encontrado)
	t.Run("POST /v1/bfcl/mortgage-loan/catalogs/Inexistente", func(t *testing.T) {
		resp, err := http.Post(server.URL+"/v1/bfcl/mortgage-loan/catalogs/Inexistente", "application/json", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusPaymentRequired {
			t.Errorf("got status %d, want 402", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		expected := `{"errors_detail":"Catálogo no encontrado"}`
		if string(body) != expected {
			t.Errorf("got %s, want %s", string(body), expected)
		}
	})

	// 10. Test POST /v1/bfcl/mortgage-loan/catalogs/TokenInvalido (401 Token no válido)
	t.Run("POST /v1/bfcl/mortgage-loan/catalogs/TokenInvalido", func(t *testing.T) {
		resp, err := http.Post(server.URL+"/v1/bfcl/mortgage-loan/catalogs/TokenInvalido", "application/json", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("got status %d, want 401", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		expected := `{"detail":"Given token not valid for any token type","code":"token_not_valid","messages":[{"token_class":"AccessToken","token_type":"access","message":"Token is invalid"}]}`
		if string(body) != expected {
			t.Errorf("got %s, want %s", string(body), expected)
		}
	})
}
