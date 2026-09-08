package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if port[0] != ':' {
		port = ":" + port
	}

	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"UP","service":"finnflow-mock"}`))
	})

	// FinnFlow catalog detail endpoint: /api/catalogo_detail/{catalog}
	mux.HandleFunc("/api/catalogo_detail/", handleCatalogDetail)

	server := &http.Server{
		Addr:         port,
		Handler:      mux,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	log.Printf("[FinnFlow Mock] Server starting on %s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[FinnFlow Mock] Failed to start server: %v", err)
	}
}

func handleCatalogDetail(w http.ResponseWriter, r *http.Request) {
	catalogName := strings.TrimPrefix(r.URL.Path, "/api/catalogo_detail/")
	catalogName = strings.TrimSpace(catalogName)

	log.Printf("[FinnFlow Mock] Request received for catalog: %q (Method=%s, RemoteAddr=%s)",
		catalogName, r.Method, r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")

	// Check for Timeout scenario
	if catalogName == "Timeout" {
		log.Printf("[FinnFlow Mock] Simulating 35s upstream timeout for catalog: %s", catalogName)
		time.Sleep(35 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"codRespuesta":3,"Mensaje":"Sin resultados.","Excepcion":"Ninguna"}]`))
		return
	}

	// Check for Unauthorized token scenario
	if catalogName == "TokenInvalido" {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"Given token not valid for any token type","code":"token_not_valid"}`))
		return
	}

	// Match scenarios
	switch catalogName {
	case "Destino":
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"codigo_adm":"1","descripcion":"Vivienda Principal"}]`))

	case "TiposDocumentos", "tiposdocumentos":
		// Upstream always returns the exact same payload containing grupo_id
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"codigo_adm":1,"descripcion":"Cédula de Identidad","grupo_id":10}]`))

	case "Comunas", "comunas":
		// Upstream always returns the exact same payload containing region_id
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"codigo_adm":"13101","descripcion":"Santiago","region_id":13}]`))

	case "Regiones", "regiones":
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"codigo_adm":"13","descripcion":"Metropolitana de Santiago"}]`))

	case "SinResultados":
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"codRespuesta":3,"Mensaje":"Sin resultados.","Excepcion":"Ninguna"}]`))

	case "Vacio":
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))

	default:
		// Seguros match (prefix case-insensitive)
		if strings.HasPrefix(strings.ToLower(catalogName), "seguro") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"IdentificadorSeguro":"1100438-01-2023-000","Descripcion":"INCENDIO - Everest","Poliza":"100438-01-2023-000","NombreCompania":"Everest","CodigoCompania":39,"CodigoTipoSeguro":0,"CorrelativoPoliza":28,"Tasa":0.22553,"Factor":2.255E-4,"IndicadorPolizaIndividual":0,"PorValorCuota":0,"IndicadorPolizaExterna":0}]`))
			return
		}

		// Unknown / Not Found -> FinnFlow 404 (Client translates to 402 "Catálogo no encontrado")
		log.Printf("[FinnFlow Mock] Catalog %q not recognized. Returning 404 Not Found", catalogName)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"Not Found"}`))
	}
}
