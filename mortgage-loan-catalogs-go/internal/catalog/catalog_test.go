package catalog_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mortgage-loan-catalogs-go/internal/catalog"
	"mortgage-loan-catalogs-go/internal/httpclient"
)

func TestCatalogServiceWithFinnFlowMockByteByByte(t *testing.T) {
	// Setup mock FinnFlow server returning exact upstream payloads
	mockFinnFlow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/catalogo_detail/Destino":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"codigo_adm":"1","descripcion":"Vivienda Principal"}]`))

		case "/api/catalogo_detail/SegurosIncendio", "/api/catalogo_detail/segurosincendio":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"IdentificadorSeguro":"1100438-01-2023-000","Descripcion":"INCENDIO - Everest","Poliza":"100438-01-2023-000","NombreCompania":"Everest","CodigoCompania":39,"CodigoTipoSeguro":0,"CorrelativoPoliza":28,"Tasa":0.22553,"Factor":2.255E-4,"IndicadorPolizaIndividual":0,"PorValorCuota":0,"IndicadorPolizaExterna":0}]`))

		case "/api/catalogo_detail/TiposDocumentos":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"codigo_adm":1,"descripcion":"Cédula de Identidad","grupo_id":10}]`))

		case "/api/catalogo_detail/tiposdocumentos":
			// Upstream returns the same raw payload as TiposDocumentos
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"codigo_adm":1,"descripcion":"Cédula de Identidad","grupo_id":10}]`))

		case "/api/catalogo_detail/Comunas":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"codigo_adm":"13101","descripcion":"Santiago","region_id":13}]`))

		case "/api/catalogo_detail/comunas":
			// Upstream returns the same raw payload as Comunas
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"codigo_adm":"13101","descripcion":"Santiago","region_id":13}]`))

		case "/api/catalogo_detail/SinResultados":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"codRespuesta":3,"Mensaje":"Sin resultados.","Excepcion":"Ninguna"}]`))

		case "/api/catalogo_detail/Vacio":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))

		case "/api/catalogo_detail/Inexistente":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"detail":"Not Found"}`))

		case "/api/catalogo_detail/TokenInvalido":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"detail":"Token is invalid"}`))

		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"codigo_adm":"DEFAULT","descripcion":"Item por defecto"}]`))
		}
	}))
	defer mockFinnFlow.Close()

	// Initialize FinnFlow client pointing to mock server
	client := httpclient.NewClient(httpclient.Config{
		BaseURL: mockFinnFlow.URL,
	})

	// Initialize Catalog service
	svc := catalog.NewService(client)

	ctx := context.Background()

	tests := []struct {
		name           string
		catalogName    string
		expectedStatus int
		expectError    bool
		expectedJSON   string
	}{
		{
			name:           "Standard catalog Destino byte-by-byte",
			catalogName:    "Destino",
			expectedStatus: http.StatusOK,
			expectError:    false,
			expectedJSON:   `[{"codigo_adm":"1","descripcion":"Vivienda Principal"}]`,
		},
		{
			name:           "Seguros with PascalCase and scientific notation factor byte-by-byte",
			catalogName:    "SegurosIncendio",
			expectedStatus: http.StatusOK,
			expectError:    false,
			expectedJSON:   `[{"IdentificadorSeguro":"1100438-01-2023-000","Descripcion":"INCENDIO - Everest","Poliza":"100438-01-2023-000","NombreCompania":"Everest","CodigoCompania":39,"CodigoTipoSeguro":0,"CorrelativoPoliza":28,"Tasa":0.22553,"Factor":2.255E-4,"IndicadorPolizaIndividual":0,"PorValorCuota":0,"IndicadorPolizaExterna":0}]`,
		},
		{
			name:           "Seguros with lowercase prefix case-insensitive match byte-by-byte",
			catalogName:    "segurosincendio",
			expectedStatus: http.StatusOK,
			expectError:    false,
			expectedJSON:   `[{"IdentificadorSeguro":"1100438-01-2023-000","Descripcion":"INCENDIO - Everest","Poliza":"100438-01-2023-000","NombreCompania":"Everest","CodigoCompania":39,"CodigoTipoSeguro":0,"CorrelativoPoliza":28,"Tasa":0.22553,"Factor":2.255E-4,"IndicadorPolizaIndividual":0,"PorValorCuota":0,"IndicadorPolizaExterna":0}]`,
		},
		{
			name:           "TiposDocumentos exact case preserves grupo_id byte-by-byte",
			catalogName:    "TiposDocumentos",
			expectedStatus: http.StatusOK,
			expectError:    false,
			expectedJSON:   `[{"codigo_adm":1,"descripcion":"Cédula de Identidad","grupo_id":10}]`,
		},
		{
			name:           "tiposdocumentos lowercase falls back to generic CatalogItem dropping grupo_id byte-by-byte",
			catalogName:    "tiposdocumentos",
			expectedStatus: http.StatusOK,
			expectError:    false,
			expectedJSON:   `[{"codigo_adm":"1","descripcion":"Cédula de Identidad"}]`,
		},
		{
			name:           "Comunas exact case preserves region_id byte-by-byte",
			catalogName:    "Comunas",
			expectedStatus: http.StatusOK,
			expectError:    false,
			expectedJSON:   `[{"codigo_adm":"13101","descripcion":"Santiago","region_id":13}]`,
		},
		{
			name:           "comunas lowercase falls back to generic CatalogItem dropping region_id byte-by-byte",
			catalogName:    "comunas",
			expectedStatus: http.StatusOK,
			expectError:    false,
			expectedJSON:   `[{"codigo_adm":"13101","descripcion":"Santiago"}]`,
		},
		{
			name:           "Legacy Sin resultados format byte-by-byte",
			catalogName:    "SinResultados",
			expectedStatus: http.StatusOK,
			expectError:    false,
			expectedJSON:   `[{"codRespuesta":3,"Mensaje":"Sin resultados.","Excepcion":"Ninguna"}]`,
		},
		{
			name:           "Empty array upstream triggers fallback Sin resultados byte-by-byte",
			catalogName:    "Vacio",
			expectedStatus: http.StatusOK,
			expectError:    false,
			expectedJSON:   `[{"codRespuesta":3,"Mensaje":"Sin resultados.","Excepcion":"Ninguna"}]`,
		},
		{
			name:           "Upstream 404 translates to client 402 with Catálogo no encontrado",
			catalogName:    "Inexistente",
			expectedStatus: http.StatusPaymentRequired,
			expectError:    true,
			expectedJSON:   "",
		},
		{
			name:           "Upstream 401 translates to client 401 with Token no válido",
			catalogName:    "TokenInvalido",
			expectedStatus: http.StatusUnauthorized,
			expectError:    true,
			expectedJSON:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, statusCode, err := svc.GetCatalog(ctx, tc.catalogName)

			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error for %s, got nil", tc.catalogName)
				}
				if statusCode != tc.expectedStatus {
					t.Errorf("got status %d, want %d", statusCode, tc.expectedStatus)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error for %s: %v", tc.catalogName, err)
			}

			if statusCode != tc.expectedStatus {
				t.Errorf("got status %d, want %d", statusCode, tc.expectedStatus)
			}

			// Validate JSON response byte-by-byte
			actualBytes, err := json.Marshal(res)
			if err != nil {
				t.Fatalf("failed to marshal result for %s: %v", tc.catalogName, err)
			}

			actualStr := string(actualBytes)
			if actualStr != tc.expectedJSON {
				t.Errorf("\nByte-by-byte mismatch for catalog %q:\ngot:  %s\nwant: %s",
					tc.catalogName, actualStr, tc.expectedJSON)
			}
		})
	}
}
