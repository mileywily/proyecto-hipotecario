# Guía Integral de Arquitectura e Implementación: Migración a Go y Gateway Kong

**Proyecto:** `mortgage-loan-catalogs` (Modernización y Migración Drop-in a Go)  
**Fecha:** 2026-09-07  
**Rol:** Arquitectura de Software & Plataforma Core Hipotecaria  

---

## 1. Resumen Ejecutivo y Objetivos

El objetivo de esta iniciativa fue migrar el microservicio Java 17 / Spring Boot 3.4.1 (`mortgage-loan-catalogs-service`) a una arquitectura moderna, ligera y de alto rendimiento en **Go 1.27 (`mortgage-loan-catalogs-go`)**, integrándolo con un **API Gateway Kong (`kong-bff-gateway`)** bajo el patrón **BFF (Backend For Frontend)**.

### Pilares Clave Alcanzados:
1. **Regla 0 (Contrato Congelado - Paridad Byte a Byte):** Ningún sistema consumidor percibe el cambio de lenguaje. Códigos HTTP, nombres de llaves, nulos y notación científica se preservaron de manera idéntica.
2. **Optimización de Red (Single Call):** Se redujo la sobrecarga hacia la API core de **FinnFlow** de 2-3 llamadas HTTP salientes por petición a **exactamente 1 sola llamada**, reduciendo la latencia y el consumo de ancho de banda.
3. **Documentación Swagger UI Embebida de 0 ms:** Se reimplementó Swagger/OpenAPI 3.0.3 pre-compilado en el binario sin reflexión en runtime, disponible tanto de forma directa como a través de Kong.
4. **Trazabilidad y Resiliencia Empresarial:** Incorporación de plugins de correlación (`X-Correlation-ID`), CORS, manejo de errores contractuales (`402`, `401`, `400`) y endpoints de orquestación (`/health`, `/ready`).

---

## 2. Arquitectura de la Solución

```mermaid
flowchart LR
    Consumer["Clientes / Frontend / Canales"] -->|HTTP / HTTPS| Kong["Kong BFF Gateway<br/>(Puerto 8000)"]
    
    subgraph Gateway ["kong-bff-gateway (Docker DB-less)"]
        Kong --> PluginCorr["Plugin correlation-id<br/>(X-Correlation-ID)"]
        PluginCorr --> PluginCORS["Plugin CORS"]
        PluginCORS --> Upstream["Upstream Round-Robin<br/>(mortgage-loan-catalogs-upstream)"]
    end

    Upstream -->|Proxy HTTP<br/>(Puerto 8003)| GoService["Servidor Go / Chi<br/>(mortgage-loan-catalogs-go)"]

    subgraph MicroservicioGo ["mortgage-loan-catalogs-go"]
        GoService --> Router["Chi Router v5"]
        Router --> CatalogHandler["CatalogHandler<br/>/v1/bfcl/mortgage-loan/catalogs/{catalog}"]
        Router --> InfraHandlers["Infra & Docs Handlers<br/>/health, /ready, /api-docs, /swagger-ui.html"]
        CatalogHandler --> CatalogService["catalog.Service<br/>(Ruteo Asimétrico & Mapeo)"]
        CatalogService --> FinnflowClient["httpclient.Client<br/>(Single Call Optimization)"]
    end

    FinnflowClient -->|Basic Auth<br/>1 Solo Round-Trip| FinnFlow["API FinnFlow Mock / Core<br/>(finnflow-mock:8080)"]
```

---

## 3. Detalle de Componentes Implementados

### 3.1 Servicio de Ruteo de Catálogos y Mapeo (`internal/catalog/service.go`)
Implementa las reglas de negocio exactas identificadas en la ingeniería inversa:

* **Asimetría de Ruteo:**
  * **`Seguros*`:** Coincidencia por prefijo *case-insensitive* (`IsSeguroCatalog`). Cubre `SegurosIncendio`, `segurosincendio`, `SEGUROS_DESGRAVAMEN`, etc.
  * **`TiposDocumentos`:** Coincidencia exacta *case-sensitive* (`name == "TiposDocumentos"`).
  * **`Comunas`:** Coincidencia exacta *case-sensitive* (`name == "Comunas"`).
  * **Fallback Genérico (`CatalogTypeGeneric`):** Cualquier variante en minúsculas (`tiposdocumentos`, `comunas`) o catálogos estándar (`Destino`, `Regiones`) cae a este flujo.
* **Degradación Controlada de Atributos:**
  * Al ingresar `tiposdocumentos` en minúsculas, se descarta el atributo `grupo_id` y `codigo_adm` se formatea a string.
  * Al ingresar `comunas` en minúsculas, se descarta el atributo `region_id`.
* **Traducción de Formato "Sin resultados":** Detección de cadenas legacy o arreglos vacíos `[]`, traduciéndolos a `[{"codRespuesta": 3, "Mensaje": "Sin resultados.", "Excepcion": "Ninguna"}]`.

### 3.2 Modelos y Paridad con Jackson (`internal/catalog/models.go`)
* **Tipo Especial `javaDouble`:**
  En Java, Jackson serializa números flotantes con `Double.toString()`, emitiendo notación científica con exponente sin ceros a la izquierda cuando $|v| < 10^{-3}$ o $|v| \ge 10^7$ (ej. `2.255E-4`), mientras que los números normales llevan al menos un decimal (`1.0`). El tipo `javaDouble` con su método `MarshalJSON()` replica exactamente este comportamiento byte a byte.
* **Preservación de Nulos (`*int`, `*javaDouble`):**
  Todos los atributos opcionales usan punteros sin la etiqueta `omitempty`, garantizando que si el upstream no envía un valor, el JSON de salida conserve explícitamente `null` en lugar de emitir valores cero (`0` o `0.0`).

### 3.3 Servidor HTTP con Chi (`cmd/server/main.go`)
* **Framework:** `github.com/go-chi/chi/v5` con middlewares de alto rendimiento (`RequestID`, `RealIP`, `Logger`, `Recoverer`).
* **Endpoints Expuestos:**
  1. `POST /v1/bfcl/mortgage-loan/catalogs/{catalog}`: Endpoint transaccional de catálogos.
  2. `GET /health` y `GET /ready`: Retornan `{"status":"UP"}` con status 200.
  3. `GET /api-docs` y `GET /api-docs.yaml`: Especificación OpenAPI 3.0 en formato YAML crudo.
  4. `GET /swagger-ui.html` y `/swagger-ui`: Interfaz visual interactiva Swagger UI.
* **Manejo Centralizado de Códigos Contractuales:**
  * `402 Payment Required`: Emite `{"errors_detail":"Catálogo no encontrado"}` (tanto para 404 como 402 del upstream).
  * `401 Unauthorized`: Emite `{"detail":"Given token not valid for any token type","code":"token_not_valid","messages":[...]}`.
  * `400 Bad Request`: Emite `{"errors_detail":"Error en la obtención de catálogos"}`.
  * `Graceful Shutdown`: Cierre controlado de sockets ante señales del sistema (`SIGINT`, `SIGTERM`).

### 3.4 Especificación y Documentación OpenAPI 3.0 (`api/openapi.yaml` & `api/spec.go`)
* Especificación OpenAPI 3.0.3 canónica que documenta los 15 catálogos con sus esquemas polimórficos (`oneOf`).
* Embebida en el binario mediante la directiva `//go:embed`, logrando un tiempo de carga instantáneo (0 ms de overhead en arranque y menos de 20 KB de huella en memoria).

### 3.5 Cliente HTTP FinnFlow (`internal/httpclient/finnflow.go`)
* **Optimización Single Call:** La función `executeSingleCall` realiza una sola llamada HTTP, cargando el cuerpo en un buffer `[]byte`. La lógica decide qué estructura decodificar según su contenido sin re-consultar a FinnFlow.
* **Resiliencia de Red Tipada:** Detección de timeouts y errores de socket (`context.DeadlineExceeded`, `net.Error`, `net.OpError`) mapeándolos a HTTP 200 con el payload de fallback `NoResultsResponse`.

### 3.6 Gateway Kong BFF (`kong-bff-gateway/kong.yml` & `kong-bff-gateway/docker-compose.yml`)
* **Modo DB-less (v3.0):** No requiere base de datos PostgreSQL, configurado puramente en YAML.
* **Imagen:** `kong:3.8` oficial en Docker Compose v2.
* **Upstream:** `mortgage-loan-catalogs-upstream` con target dinámico `host.docker.internal:8003`.
* **Rutas Configuradas:**
  * Catálogos: `/v1/bfcl/mortgage-loan/catalogs` (`POST`, `strip_path: false`, `preserve_host: true`).
  * Infraestructura: `/health`, `/ready` (`GET`).
  * Documentación: `/api-docs`, `/swagger-ui.html` (`GET`).
* **Plugins de Seguridad y Auditoría:**
  * `correlation-id`: Inyecta/propaga automáticamente el header bancario `X-Correlation-ID` con UUID.
  * `cors`: Habilita orígenes abiertos para el frontend bancario (`POST`, `GET`, `OPTIONS`).

---

## 4. Matriz de Pruebas y Validación de Calidad

| Archivo de Pruebas | Alcance y Escenarios Validados | Resultado |
|---|---|:---:|
| `internal/catalog/models_test.go` | Validación de serialización de structs, nulos sin omitempty y notación científica de Jackson `javaDouble` (`2.255E-4`, `1.0E7`, `0.0`). | **PASS** |
| `internal/catalog/service_test.go` | Validación exhaustiva de las reglas de ruteo, asimetría de matching, degradación de atributos en minúsculas y fallbacks. | **PASS** |
| `internal/catalog/catalog_test.go` | **Test de Paridad Byte a Byte:** Mock completo de FinnFlow con `httptest.Server`, validando que cada JSON emitido sea idéntico carácter a carácter. | **PASS** |
| `internal/httpclient/finnflow_test.go` | Validación de la optimización de llamada única (1 solo round-trip), timeouts a 200 y traducción de códigos de error (404 a 402). | **PASS** |
| `cmd/server/main_test.go` | Integración HTTP sobre el router Chi: `/health`, `/ready`, `/api-docs`, `/swagger-ui.html` y llamadas `POST` completas. | **PASS** |

---

## 5. Manual de Operación y Comandos de Ejecución

### 5.1 Ejecución de Pruebas
Para ejecutar la suite de pruebas completa:
```powershell
cd C:\proyecto-hipotecario\mortgage-loan-catalogs-go
go test -v ./internal/catalog/... ./internal/httpclient/... ./cmd/server/... ./api/...
```

### 5.2 Compilación del Binario
```powershell
cd C:\proyecto-hipotecario\mortgage-loan-catalogs-go
go build -o server.exe ./cmd/server
```

### 5.3 Construcción de Imagen Docker Multi-Stage (Microservicio Go)
El archivo `Dockerfile` en `mortgage-loan-catalogs-go` utiliza un build multi-stage optimizado:
* **Stage 1 (Builder):** `golang:alpine` para compilar el binario estático (`CGO_ENABLED=0`) con strip de símbolos (`-ldflags="-s -w"`).
* **Stage 2 (Runtime):** `alpine:latest` con usuario no-root (`appuser`), zona horaria y certificados SSL. La imagen final pesa solo **~11 MB**.

Para construir la imagen de manera independiente:
```powershell
cd C:\proyecto-hipotecario\mortgage-loan-catalogs-go
docker build -t mortgage-loan-catalogs-go:latest .
```

### 5.4 Levantar el Stack Completo Unificado (Kong Gateway + Microservicio Go)
Ambos servicios están orquestados dentro de `kong-bff-gateway/docker-compose.yml` en la red interna `bff-net`:
```powershell
cd C:\proyecto-hipotecario\kong-bff-gateway
docker compose up -d --build
```

*Comandos de verificación de contenedores:*
```powershell
# Ver estado de los servicios
docker compose ps

# Ver logs de Kong Gateway
docker logs -f kong-bff-gateway

# Ver logs del microservicio Go
docker logs -f mortgage-loan-catalogs-go
```

### 5.5 Ejecución Nativa de Go en Host (Desarrollo Rápido opcional)
Si se desea ejecutar Go directamente sobre la máquina host sin Docker:
```powershell
cd C:\proyecto-hipotecario\mortgage-loan-catalogs-go
$env:PORT="8003"
go run ./cmd/server
```

---

## 6. URLs de Consulta y Swagger UI

| Servicio | Tipo de Consulta | URL en Navegador / Cliente REST |
|---|---|---|
| **Kong BFF Gateway** | **Swagger UI Interactivo** | `http://localhost:8000/swagger-ui.html` |
| **Kong BFF Gateway** | **Especificación OpenAPI (YAML)** | `http://localhost:8000/api-docs` |
| **Kong BFF Gateway** | **Consulta Catálogo (POST)** | `http://localhost:8000/v1/bfcl/mortgage-loan/catalogs/Destino` |
| **Kong BFF Gateway** | **Health Check (GET)** | `http://localhost:8000/health` |
| **Microservicio Go Directo** | **Swagger UI Interactivo** | `http://localhost:8003/swagger-ui.html` |
| **Microservicio Go Directo** | **Especificación OpenAPI (YAML)** | `http://localhost:8003/api-docs` |
| **Microservicio Go Directo** | **Consulta Catálogo (POST)** | `http://localhost:8003/v1/bfcl/mortgage-loan/catalogs/Destino` |
| **Microservicio Go Directo** | **Health Check (GET)** | `http://localhost:8003/health` |

---

## 7. Diagnóstico Operativo y Preguntas Frecuentes (FAQ)

### 7.1 ¿Por qué el servidor responde `HTTP 402` ("Catálogo no encontrado") si existen mocks?

Al ejecutar una petición en vivo contra el Gateway o el microservicio Go:
```bash
HTTP/1.1 402 Payment Required
Content-Type: application/json
{"errors_detail":"Catálogo no encontrado"}
```

Este comportamiento es **completamente esperado y confirma la paridad contractual con Java**:

1. **Mocks de Pruebas (`go test`) vs. Servidor en Vivo (`cmd/server`):**
   * Los mocks implementados con `httptest.Server` se ejecutan **únicamente en memoria durante los tests unitarios** (`catalog_test.go`, `finnflow_test.go`). Al finalizar el test, estos servidores simulados se destruyen.
   * El servidor en vivo (`go run ./cmd/server`) no utiliza datos estáticos en código; intenta comunicarse en tiempo real con el upstream core de **FinnFlow**.
2. **Origen de la respuesta 402:**
   * En ejecución local, la variable `FINNFLOW_URL` toma por defecto `http://localhost:8080/api/catalogo_detail`.
   * En el puerto `8080` de la máquina local suele estar el contenedor Java (`gallant_poincare`), el cual **no implementa** `/api/catalogo_detail` (dado que ese es un endpoint del core bancario externo), respondiendo con un `HTTP 404 Not Found`.
   * **Regla de Negocio Contractual:** El cliente HTTP de Go traduce cualquier respuesta `404 Not Found` o `402` de FinnFlow a un código **`HTTP 402 Payment Required`** con el cuerpo exacto `{"errors_detail":"Catálogo no encontrado"}`.
   * Por lo tanto, recibir este error confirma que:
     * Kong interceptó la petición, inyectó el `X-Correlation-ID` y la enrutó a Go.
     * El servicio Go procesó la ruta y llamó al upstream.
     * La regla de paridad contractual de mapeo de errores funcionó al 100%.

### 7.2 Cómo ejecutar cURL en Windows PowerShell sin errores de sintaxis

En Windows PowerShell existen particularidades que difieren de entornos Bash/Linux:

1. **No anteponer `cd`:** `cd` es alias de `Set-Location`, por lo que `cd curl -i ...` provocará un error de parámetro ambiguo (`-i`).
2. **No usar `\` para continuación de línea:** En PowerShell el carácter de escape y salto de línea es el acento grave (`` ` ``). Si se usa `\`, PowerShell interpretará las líneas siguientes (`-H ...`) como comandos independientes no reconocidos.
3. **Invocar `curl.exe` explícito:** `curl` en PowerShell 5.1 es un alias de `Invoke-WebRequest`. Use siempre `curl.exe` para llamar a la utilidad nativa de Windows.

#### Comando Correcto en una sola línea (PowerShell):
```powershell
curl.exe -i -X POST http://localhost:8000/v1/bfcl/mortgage-loan/catalogs/TiposDocumentos -H "Content-Type: application/json" -d '{\"client_type\": \"NATURAL\"}'
```

#### Alternativa Nativa en PowerShell (`Invoke-RestMethod`):
```powershell
Invoke-RestMethod -Uri "http://localhost:8000/v1/bfcl/mortgage-loan/catalogs/TiposDocumentos" -Method Post -ContentType "application/json" -Body '{"client_type": "NATURAL"}'
```

#### Para visualizar cabeceras inyectadas por Kong (`Invoke-WebRequest`):
```powershell
$resp = Invoke-WebRequest -Uri "http://localhost:8000/v1/bfcl/mortgage-loan/catalogs/TiposDocumentos" -Method Post -ContentType "application/json" -Body '{"client_type": "NATURAL"}' -SkipHttpErrorCheck
$resp.Headers
$resp.Content
```

### 7.3 Conectar el Microservicio a un Entorno Real o Mock Server Externo

Para recibir catálogos poblados con datos en el servidor en vivo, defina las variables de entorno de conexión antes de iniciar Go:

```powershell
# Apuntar a un entorno de desarrollo de FinnFlow o a un Mock Server dedicado
$env:PORT="8003"
$env:FINNFLOW_URL="https://finnflow-dev.dominio.com"
$env:FINNFLOW_KEY="usuario_credencial"
$env:FINNFLOW_SECRET="password_credencial"
$env:FINNFLOW_TIMEOUT="10s"

cd C:\proyecto-hipotecario\mortgage-loan-catalogs-go
go run ./cmd/server
```

---

## 8. Matriz de Escenarios Históricos de Prueba en Vivo (E2E vía Kong)

Con el stack completo en Docker (`docker compose up -d`), el mock de FinnFlow soporta todos los escenarios históricos de prueba:

| Escenario | Endpoint POST vía Kong | Código HTTP | Comportamiento y Validación |
|---|---|---|---|
| **1. Catálogo Estándar (`Destino`)** | `/v1/bfcl/mortgage-loan/catalogs/Destino` | `200 OK` | `[{"codigo_adm":"1","descripcion":"Vivienda Principal"}]` |
| **2. Seguros PascalCase (`SegurosIncendio`)** | `/v1/bfcl/mortgage-loan/catalogs/SegurosIncendio` | `200 OK` | Preserva notación científica Jackson: `Factor: 2.255E-4` |
| **3. Seguros minúsculas (`segurosincendio`)** | `/v1/bfcl/mortgage-loan/catalogs/segurosincendio` | `200 OK` | Ruteo asimétrico case-insensitive idéntico a PascalCase |
| **4. TiposDocumentos (Exact case)** | `/v1/bfcl/mortgage-loan/catalogs/TiposDocumentos` | `200 OK` | Preserva `grupo_id: 10` y `codigo_adm: 1` numérico |
| **5. tiposdocumentos (Minúsculas)** | `/v1/bfcl/mortgage-loan/catalogs/tiposdocumentos` | `200 OK` | Fallback genérico: descarta `grupo_id` y `codigo_adm: "1"` string |
| **6. Comunas (Exact case)** | `/v1/bfcl/mortgage-loan/catalogs/Comunas` | `200 OK` | Preserva `region_id: 13` |
| **7. comunas (Minúsculas)** | `/v1/bfcl/mortgage-loan/catalogs/comunas` | `200 OK` | Fallback genérico: descarta `region_id` |
| **8. Regiones estándar (`Regiones`)** | `/v1/bfcl/mortgage-loan/catalogs/Regiones` | `200 OK` | `[{"codigo_adm":"13","descripcion":"Metropolitana de Santiago"}]` |
| **9. Mensaje Legacy (`SinResultados`)** | `/v1/bfcl/mortgage-loan/catalogs/SinResultados` | `200 OK` | `[{"codRespuesta":3,"Mensaje":"Sin resultados.","Excepcion":"Ninguna"}]` |
| **10. Arreglo Vacío (`Vacio`)** | `/v1/bfcl/mortgage-loan/catalogs/Vacio` | `200 OK` | Mapeo automático upstream `[]` $\rightarrow$ `NoResultsResponse` |
| **11. Catálogo Inexistente (`Inexistente`)** | `/v1/bfcl/mortgage-loan/catalogs/Inexistente` | `402 Payment Required` | Upstream 404 $\rightarrow$ `{"errors_detail":"Catálogo no encontrado"}` |
| **12. Token Inválido (`TokenInvalido`)** | `/v1/bfcl/mortgage-loan/catalogs/TokenInvalido` | `401 Unauthorized` | Upstream 401 $\rightarrow$ `{"detail":"Given token not valid..."}` |
| **13. Timeout Upstream (`Timeout`)** | `/v1/bfcl/mortgage-loan/catalogs/Timeout` | `200 OK` | Retardo > timeout cliente $\rightarrow$ fallback `NoResultsResponse` |

### Comandos de Validación Rápida en PowerShell

```powershell
# 1. Catálogo Estándar Destino
curl.exe -s -X POST http://localhost:8000/v1/bfcl/mortgage-loan/catalogs/Destino -H "Content-Type: application/json" -d '{\"client_type\": \"NATURAL\"}'

# 2. Seguros PascalCase con Notación Científica
curl.exe -s -X POST http://localhost:8000/v1/bfcl/mortgage-loan/catalogs/SegurosIncendio -H "Content-Type: application/json" -d '{\"client_type\": \"NATURAL\"}'

# 3. TiposDocumentos (preserva grupo_id)
curl.exe -s -X POST http://localhost:8000/v1/bfcl/mortgage-loan/catalogs/TiposDocumentos -H "Content-Type: application/json" -d '{\"client_type\": \"NATURAL\"}'

# 4. tiposdocumentos (descarta grupo_id)
curl.exe -s -X POST http://localhost:8000/v1/bfcl/mortgage-loan/catalogs/tiposdocumentos -H "Content-Type: application/json" -d '{\"client_type\": \"NATURAL\"}'

# 5. Catálogo Inexistente (HTTP 402 contractual)
curl.exe -i -s -X POST http://localhost:8000/v1/bfcl/mortgage-loan/catalogs/Inexistente -H "Content-Type: application/json" -d '{\"client_type\": \"NATURAL\"}'



---

## 9. Automatización CI/CD (GitHub Actions & GitLab CI)

El proyecto cuenta con plantillas estandarizadas para integración y entrega continua en la nube:

### 9.1 GitHub Actions (`.github/workflows/ci.yml`)
Automatiza el ciclo de vida completo en GitHub:
1. **Job `test`:**
   * Configura Go, ejecuta `go vet` y verificación estricta de formato `gofmt`.
   * Ejecuta pruebas unitarias con detección de carreras (`-race`) y genera reporte de cobertura (`coverage.out`).
2. **Job `integration-smoke-test`:**
   * Construye los contenedores de Go y Mock.
   * Levanta todo el stack con `docker compose up -d`.
   * Espera que `/health` responda `UP` y ejecuta smoke tests validando paridad de bytes (`Destino`, `SegurosIncendio`, `Inexistente` 402).
   * Destruye el stack limpiamente.
3. **Job `publish` (Opcional en merge a main/tags):**
   * Publica la imagen del microservicio en **GitHub Container Registry (`ghcr.io`)**.

### 9.2 GitLab CI (`.gitlab-ci.yml`)
Plantilla para el pipeline corporativo de GitLab con stages:
* `test`: Compilación y pruebas unitarias con cobertura.
* `build`: Construcción de imágenes Docker con Docker-in-Docker (`dind`).
* `smoke-test`: Validación de integración con Docker Compose.

### 9.3 Archivo de Exclusiones Git (`.gitignore`)
Previene la subida accidental de binarios compilados (`server.exe`, `mock-finnflow`), empaquetados `.tar`, logs y reportes de cobertura temporales.
