package catalog

import (
	"math"
	"strconv"
	"strings"
)

// javaDouble represents a float64 that serializes to JSON matching Java Jackson's Double.toString behavior.
// When magnitude is < 1e-3 (and != 0) or >= 1e7, it serializes in scientific notation with uppercase 'E'
// and no leading zeros in the exponent (e.g. 2.255E-4, 1.0E7), matching Jackson byte-for-byte.
type javaDouble float64

// JavaDouble is an exported alias for javaDouble.
type JavaDouble = javaDouble

// NewJavaDouble returns a pointer to a javaDouble.
func NewJavaDouble(v float64) *javaDouble {
	d := javaDouble(v)
	return &d
}

// Float64 returns the underlying float64 value, or 0 if nil.
func (d *javaDouble) Float64() float64 {
	if d == nil {
		return 0
	}
	return float64(*d)
}

// MarshalJSON serializes the javaDouble to match Java's Double.toString / Jackson formatting.
func (d javaDouble) MarshalJSON() ([]byte, error) {
	return []byte(formatJavaDouble(float64(d))), nil
}

// UnmarshalJSON deserializes numbers or null into javaDouble.
func (d *javaDouble) UnmarshalJSON(data []byte) error {
	s := strings.TrimSpace(string(data))
	if s == "null" || s == `""` {
		return nil
	}
	s = strings.Trim(s, `"`)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*d = javaDouble(f)
	return nil
}

// formatJavaDouble reproduces Java's java.lang.Double.toString(double d) specification.
func formatJavaDouble(f float64) string {
	if math.IsNaN(f) {
		return "NaN"
	}
	if math.IsInf(f, 1) {
		return "Infinity"
	}
	if math.IsInf(f, -1) {
		return "-Infinity"
	}
	if f == 0 {
		if math.Signbit(f) {
			return "-0.0"
		}
		return "0.0"
	}

	abs := math.Abs(f)
	if abs >= 1e-3 && abs < 1e7 {
		s := strconv.FormatFloat(f, 'f', -1, 64)
		if !strings.Contains(s, ".") {
			s += ".0"
		}
		return s
	}

	// Scientific notation: single-digit integer part + '.' + fractional part + 'E' + signed exponent without leading zeros
	s := strconv.FormatFloat(f, 'E', -1, 64)
	eIdx := strings.Index(s, "E")
	if eIdx != -1 {
		prefix := s[:eIdx]
		suffix := s[eIdx+1:]
		if !strings.Contains(prefix, ".") {
			prefix += ".0"
		}
		exp, err := strconv.Atoi(suffix)
		if err == nil {
			suffix = strconv.Itoa(exp)
		}
		s = prefix + "E" + suffix
	}
	return s
}

// CatalogItem represents standard catalog items (key-value pair: codigo_adm, descripcion).
type CatalogItem struct {
	CodigoAdm   string `json:"codigo_adm"`
	Descripcion string `json:"descripcion"`
}

// ComunaItem represents comuna catalog items with region association.
type ComunaItem struct {
	CodigoAdm   string `json:"codigo_adm"`
	Descripcion string `json:"descripcion"`
	RegionId    *int   `json:"region_id"`
}

// ToCatalogItem converts ComunaItem to a standard CatalogItem (dropping region_id).
func (c ComunaItem) ToCatalogItem() CatalogItem {
	return CatalogItem{
		CodigoAdm:   c.CodigoAdm,
		Descripcion: c.Descripcion,
	}
}

// TipoDocumentoItem represents document type catalog items with grupo_id.
// Note: codigo_adm is an Integer (*int) in TipoDocumentoItem, unlike CatalogItem where it is a String.
type TipoDocumentoItem struct {
	CodigoAdm   *int   `json:"codigo_adm"`
	Descripcion string `json:"descripcion"`
	GrupoId     *int   `json:"grupo_id"`
}

// ToCatalogItem converts TipoDocumentoItem to a standard CatalogItem (formatting codigo_adm to string and dropping grupo_id).
func (t TipoDocumentoItem) ToCatalogItem() CatalogItem {
	var codStr string
	if t.CodigoAdm != nil {
		codStr = strconv.Itoa(*t.CodigoAdm)
	}
	return CatalogItem{
		CodigoAdm:   codStr,
		Descripcion: t.Descripcion,
	}
}

// SeguroItem represents insurance catalog items.
// Uses pointers (*int, *javaDouble) to preserve null values without omitempty.
type SeguroItem struct {
	IdentificadorSeguro       string      `json:"IdentificadorSeguro"`
	Descripcion               string      `json:"Descripcion"`
	Poliza                    string      `json:"Poliza"`
	NombreCompania            string      `json:"NombreCompania"`
	CodigoCompania            *int        `json:"CodigoCompania"`
	CodigoTipoSeguro          *int        `json:"CodigoTipoSeguro"`
	CorrelativoPoliza         *int        `json:"CorrelativoPoliza"`
	Tasa                      *javaDouble `json:"Tasa"`
	Factor                    *javaDouble `json:"Factor"`
	IndicadorPolizaIndividual *int        `json:"IndicadorPolizaIndividual"`
	PorValorCuota             *int        `json:"PorValorCuota"`
	IndicadorPolizaExterna    *int        `json:"IndicadorPolizaExterna"`
}

// NoResultsResponse represents the fallback response when a catalog has no results or network errors occur.
type NoResultsResponse struct {
	CodRespuesta int    `json:"codRespuesta"`
	Mensaje      string `json:"Mensaje"`
	Excepcion    string `json:"Excepcion"`
}

// NewNoResultsResponse returns a NoResultsResponse initialized with default values matching the Java DTO.
func NewNoResultsResponse() NoResultsResponse {
	return NoResultsResponse{
		CodRespuesta: 3,
		Mensaje:      "Sin resultados.",
		Excepcion:    "Ninguna",
	}
}

// PtrInt returns a pointer to an int value.
func PtrInt(v int) *int {
	return &v
}
