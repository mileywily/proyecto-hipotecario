package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"mortgage-loan-catalogs-go/api"
	"mortgage-loan-catalogs-go/internal/catalog"
	"mortgage-loan-catalogs-go/internal/httpclient"
)

// SetupRouter creates and configures the chi router with all endpoints and middleware.
func SetupRouter(svc *catalog.Service) *chi.Mux {
	r := chi.NewRouter()

	// Base middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Infrastructure endpoints
	r.Get("/health", HealthHandler)
	r.Get("/ready", ReadyHandler)

	// Swagger / OpenAPI documentation endpoints (parity with SpringDoc URLs)
	r.Get("/api-docs", APIDocsHandler)
	r.Get("/api-docs.yaml", APIDocsHandler)
	r.Get("/swagger-ui.html", SwaggerUIHandler)
	r.Get("/swagger-ui", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger-ui.html", http.StatusMovedPermanently)
	})
	r.Get("/swagger-ui/*", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger-ui.html", http.StatusMovedPermanently)
	})

	// Domain endpoint: POST /v1/bfcl/mortgage-loan/catalogs/{catalog}
	r.Post("/v1/bfcl/mortgage-loan/catalogs/{catalog}", CatalogHandler(svc))

	return r
}

// HealthHandler responds with the service health status.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"UP"}`))
}

// ReadyHandler responds with the service readiness status.
func ReadyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"UP"}`))
}

// APIDocsHandler serves the raw OpenAPI 3.0 specification in YAML format.
func APIDocsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-yaml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(api.OpenAPISpec)
}

// SwaggerUIHandler serves the interactive Swagger UI interface.
func SwaggerUIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	const html = `<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="UTF-8">
  <title>Swagger UI - Mortgage Loan Catalogs API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <link rel="icon" type="image/png" href="https://unpkg.com/swagger-ui-dist@5/favicon-32x32.png" sizes="32x32" />
  <style>
    html { box-sizing: border-box; overflow: -moz-scrollbars-vertical; overflow-y: scroll; }
    *, *:before, *:after { box-sizing: inherit; }
    body { margin:0; background: #fafafa; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
  <script>
    window.onload = function() {
      window.ui = SwaggerUIBundle({
        url: "/api-docs",
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        layout: "BaseLayout"
      });
    };
  </script>
</body>
</html>`
	_, _ = w.Write([]byte(html))
}

// CatalogHandler handles POST /v1/bfcl/mortgage-loan/catalogs/{catalog} requests.
func CatalogHandler(svc *catalog.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		catalogName := chi.URLParam(r, "catalog")
		if catalogName == "" {
			writeErrorResponse(w, http.StatusBadRequest, "Catálogo no especificado")
			return
		}

		res, statusCode, err := svc.GetCatalog(r.Context(), catalogName)
		if err != nil {
			handleCatalogError(w, statusCode, err)
			return
		}

		jsonData, err := json.Marshal(res)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, "Error al serializar respuesta")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write(jsonData)
	}
}

// handleCatalogError maps domain errors to the exact JSON error responses matching the Java contract.
func handleCatalogError(w http.ResponseWriter, statusCode int, err error) {
	w.Header().Set("Content-Type", "application/json")

	switch statusCode {
	case http.StatusPaymentRequired: // 402: Catálogo no encontrado (contractual)
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"errors_detail":"Catálogo no encontrado"}`))

	case http.StatusUnauthorized: // 401: Token no válido
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"Given token not valid for any token type","code":"token_not_valid","messages":[{"token_class":"AccessToken","token_type":"access","message":"Token is invalid"}]}`))

	case http.StatusBadRequest: // 400: Error en la obtención de catálogos
		w.WriteHeader(http.StatusBadRequest)
		msg := err.Error()
		if msg == "" {
			msg = "Error en la obtención de catálogos"
		}
		writeJSONError(w, msg)

	default:
		if statusCode <= 0 {
			statusCode = http.StatusBadRequest
		}
		w.WriteHeader(statusCode)
		writeJSONError(w, err.Error())
	}
}

func writeErrorResponse(w http.ResponseWriter, statusCode int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	writeJSONError(w, msg)
}

func writeJSONError(w http.ResponseWriter, msg string) {
	resp := struct {
		ErrorsDetail string `json:"errors_detail"`
	}{
		ErrorsDetail: msg,
	}
	data, _ := json.Marshal(resp)
	_, _ = w.Write(data)
}

func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("SERVER_PORT")
	}
	if port == "" {
		port = "8080"
	}
	if port[0] != ':' {
		port = ":" + port
	}
	return port
}

func main() {
	// Initialize FinnFlow client configuration from environment
	cfg := httpclient.DefaultConfig()
	client := httpclient.NewClient(cfg)

	// Initialize Catalog service
	svc := catalog.NewService(client)

	// Setup router
	router := SetupRouter(svc)

	port := getPort()
	server := &http.Server{
		Addr:         port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Channel to listen for shutdown signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Starting mortgage-loan-catalogs server on %s", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
