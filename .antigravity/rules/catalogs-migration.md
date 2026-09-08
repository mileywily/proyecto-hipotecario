# Antigravity Rules: Migration mortgage-loan-catalogs to Go

## Regla 0: Contrato congelado (Paridad byte a byte)
- La respuesta JSON del servicio Go debe ser IDÉNTICA a la del servicio Java actual.
- Mantener la asimetría de matching:
  - `Seguros*`: case-insensitive por prefijo.
  - `TiposDocumentos` y `Comunas`: case-sensitive exacto (si entra `tiposdocumentos` en minúsculas debe perder `grupo_id`).
- Mapeo de status HTTP Upstream (FinnFlow):
  - 401 Upstream -> 401 Client
  - 404 Upstream -> 402 Client ("Catálogo no encontrado")
  - Timeout / Error de Red -> 200 Client con `[{"codRespuesta": 3, "Mensaje": "Sin resultados.", "Excepcion": "Ninguna"}]`

## Diseño Go & Punteros Nullable
- Usar `net/http` + `chi` para el router.
- Usar punteros (`*int`, `*float64`) para todos los campos opcionales/wrappers Java para preservar `null` sin usar `omitempty`.
- Respetar el orden exacto de los campos del struct según los DTOs Java.

## Optimización Clave
- Realizar ÚNICAMENTE 1 llamada HTTP a FinnFlow por request (leer el body completo a `[]byte` y decidir el struct a decodificar según su contenido), en lugar de las 2-3 llamadas que hace Java.
