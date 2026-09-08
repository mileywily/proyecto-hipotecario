# Script de Comparación de Paridad Java (8080) vs Go (8081)

$javaUrl = "http://localhost:8080/v1/bfcl/mortgage-loan/catalogs"
$goUrl   = "http://localhost:8081/v1/bfcl/mortgage-loan/catalogs"

$catalogs = @(
    "Destino",
    "Objetivo",
    "Comunas",
    "TiposDocumentos",
    "SegurosIncendio",
    "SegurosDesgravamen",
    "tiposdocumentos", # Caso trampa: minúsculas
    "comunas",         # Caso trampa: minúsculas
    "NoExiste"         # Caso error 402/404
)

Write-Host "==================================================" -ForegroundColor Cyan
Write-Host " INICIANDO PRUEBAS DE PARIDAD JAVA VS GO" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan

function Get-HttpResponse($url) {
    try {
        $res = Invoke-WebRequest -Uri $url -Method POST -UseBasicParsing -ErrorAction Stop
        return @{
            Status = [int]$res.StatusCode
            Body   = $res.Content
        }
    } catch {
        if ($_.Exception.Response) {
            $resp = $_.Exception.Response
            $status = [int]$resp.StatusCode
            $stream = $resp.GetResponseStream()
            if ($null -ne $stream) {
                $reader = New-Object System.IO.StreamReader($stream, [System.Text.Encoding]::UTF8)
                $body = $reader.ReadToEnd()
            } else {
                $body = ""
            }
            return @{
                Status = $status
                Body   = $body
            }
        } else {
            return @{
                Status = 0
                Body   = "Error de conexión: $($_.Exception.Message)"
            }
        }
    }
}

foreach ($cat in $catalogs) {
    Write-Host "`n[+] Probando Catálogo: $cat" -ForegroundColor Yellow

    $java = Get-HttpResponse "$javaUrl/$cat"
    $go   = Get-HttpResponse "$goUrl/$cat"

    $statusJava = $java.Status
    $statusGo   = $go.Status
    $bodyJava   = $java.Body
    $bodyGo     = $go.Body

    # Comparación de Status
    if ($statusJava -eq $statusGo) {
        Write-Host "  [OK] Status Code Coincide: $statusJava" -ForegroundColor Green
    } else {
        Write-Host "  [FAIL] Status Code Desalineado! Java: $statusJava | Go: $statusGo" -ForegroundColor Red
    }

    # Comparación de Cuerpos (Byte a Byte)
    if ($bodyJava -eq $bodyGo) {
        Write-Host "  [OK] Cuerpos JSON Idénticos (Paridad Byte a Byte)" -ForegroundColor Green
    } else {
        Write-Host "  [FAIL] Cuerpos JSON Diferentes!" -ForegroundColor Red
        Write-Host "   -> Java ($statusJava): $bodyJava" -ForegroundColor Gray
        Write-Host "   -> Go   ($statusGo): $bodyGo"   -ForegroundColor Gray
    }
}
