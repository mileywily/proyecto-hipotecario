package catalog

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCatalogItemSerialization(t *testing.T) {
	item := CatalogItem{
		CodigoAdm:   "1",
		Descripcion: "Vivienda Principal",
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("unexpected error marshaling CatalogItem: %v", err)
	}

	expected := `{"codigo_adm":"1","descripcion":"Vivienda Principal"}`
	if string(data) != expected {
		t.Errorf("got %s, want %s", string(data), expected)
	}
}

func TestComunaItemSerialization(t *testing.T) {
	regionId := 13
	item := ComunaItem{
		CodigoAdm:   "13101",
		Descripcion: "Santiago",
		RegionId:    &regionId,
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("unexpected error marshaling ComunaItem: %v", err)
	}

	expected := `{"codigo_adm":"13101","descripcion":"Santiago","region_id":13}`
	if string(data) != expected {
		t.Errorf("got %s, want %s", string(data), expected)
	}

	// Test null preservation
	itemNull := ComunaItem{
		CodigoAdm:   "13101",
		Descripcion: "Santiago",
		RegionId:    nil,
	}
	dataNull, err := json.Marshal(itemNull)
	if err != nil {
		t.Fatalf("unexpected error marshaling ComunaItem with nil RegionId: %v", err)
	}

	expectedNull := `{"codigo_adm":"13101","descripcion":"Santiago","region_id":null}`
	if string(dataNull) != expectedNull {
		t.Errorf("got %s, want %s", string(dataNull), expectedNull)
	}

	// Test conversion to CatalogItem
	catItem := item.ToCatalogItem()
	if catItem.CodigoAdm != "13101" || catItem.Descripcion != "Santiago" {
		t.Errorf("unexpected conversion: %+v", catItem)
	}
}

func TestTipoDocumentoItemSerialization(t *testing.T) {
	codAdm := 1
	grupoId := 10
	item := TipoDocumentoItem{
		CodigoAdm:   &codAdm,
		Descripcion: "Cédula de Identidad",
		GrupoId:     &grupoId,
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("unexpected error marshaling TipoDocumentoItem: %v", err)
	}

	expected := `{"codigo_adm":1,"descripcion":"Cédula de Identidad","grupo_id":10}`
	if string(data) != expected {
		t.Errorf("got %s, want %s", string(data), expected)
	}

	// Test null preservation
	itemNull := TipoDocumentoItem{
		CodigoAdm:   nil,
		Descripcion: "Documento Especial",
		GrupoId:     nil,
	}
	dataNull, err := json.Marshal(itemNull)
	if err != nil {
		t.Fatalf("unexpected error marshaling TipoDocumentoItem with nil fields: %v", err)
	}

	expectedNull := `{"codigo_adm":null,"descripcion":"Documento Especial","grupo_id":null}`
	if string(dataNull) != expectedNull {
		t.Errorf("got %s, want %s", string(dataNull), expectedNull)
	}

	// Test conversion to CatalogItem
	catItem := item.ToCatalogItem()
	if catItem.CodigoAdm != "1" || catItem.Descripcion != "Cédula de Identidad" {
		t.Errorf("unexpected conversion: %+v", catItem)
	}
}

func TestJavaDoubleJacksonScientificNotation(t *testing.T) {
	testCases := []struct {
		name     string
		input    float64
		expected string
	}{
		{"Zero", 0.0, "0.0"},
		{"Standard decimal tasa", 0.22553, "0.22553"},
		{"Standard decimal integer", 1.0, "1.0"},
		{"Standard decimal fraction", 12345.67, "12345.67"},
		{"Threshold decimal 1e-3", 0.001, "0.001"},
		{"Jackson scientific factor 0.0002255", 0.0002255, "2.255E-4"},
		{"Jackson scientific below 1e-3", 0.000999, "9.99E-4"},
		{"Jackson scientific small number", 0.00000012345, "1.2345E-7"},
		{"Jackson scientific negative", -0.0002255, "-2.255E-4"},
		{"Jackson scientific threshold 1e7", 10000000.0, "1.0E7"},
		{"Jackson scientific large number", 12345678.9, "1.23456789E7"},
		{"Jackson scientific negative large", -10000000.0, "-1.0E7"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d := javaDouble(tc.input)
			b, err := d.MarshalJSON()
			if err != nil {
				t.Fatalf("unexpected error marshaling javaDouble: %v", err)
			}
			if string(b) != tc.expected {
				t.Errorf("got %s, want %s", string(b), tc.expected)
			}
		})
	}
}

func TestJavaDoubleUnmarshal(t *testing.T) {
	testCases := []struct {
		json     string
		expected float64
	}{
		{`0.0002255`, 0.0002255},
		{`2.255E-4`, 0.0002255},
		{`"2.255E-4"`, 0.0002255},
		{`0.22553`, 0.22553},
		{`1.0E7`, 10000000.0},
	}

	for _, tc := range testCases {
		var d javaDouble
		if err := json.Unmarshal([]byte(tc.json), &d); err != nil {
			t.Fatalf("failed to unmarshal %s: %v", tc.json, err)
		}
		if d.Float64() != tc.expected {
			t.Errorf("unmarshaled %s got %v, want %v", tc.json, d.Float64(), tc.expected)
		}
	}
}

func TestSeguroItemSerialization(t *testing.T) {
	// Replicating exact test case from CatalogServiceTest.java
	// new SeguroItem("1100438-01-2023-000", "INCENDIO - Everest", "100438-01-2023-000",
	//                "Everest", 39, 0, 28, 0.22553, 0.0002255, 0, 0, 0)
	item := SeguroItem{
		IdentificadorSeguro:       "1100438-01-2023-000",
		Descripcion:               "INCENDIO - Everest",
		Poliza:                    "100438-01-2023-000",
		NombreCompania:            "Everest",
		CodigoCompania:            PtrInt(39),
		CodigoTipoSeguro:          PtrInt(0),
		CorrelativoPoliza:         PtrInt(28),
		Tasa:                      NewJavaDouble(0.22553),
		Factor:                    NewJavaDouble(0.0002255),
		IndicadorPolizaIndividual: PtrInt(0),
		PorValorCuota:             PtrInt(0),
		IndicadorPolizaExterna:    PtrInt(0),
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("unexpected error marshaling SeguroItem: %v", err)
	}

	expected := `{"IdentificadorSeguro":"1100438-01-2023-000","Descripcion":"INCENDIO - Everest","Poliza":"100438-01-2023-000","NombreCompania":"Everest","CodigoCompania":39,"CodigoTipoSeguro":0,"CorrelativoPoliza":28,"Tasa":0.22553,"Factor":2.255E-4,"IndicadorPolizaIndividual":0,"PorValorCuota":0,"IndicadorPolizaExterna":0}`
	if string(data) != expected {
		t.Errorf("\ngot:  %s\nwant: %s", string(data), expected)
	}

	// Verify exact field sequence
	fieldOrder := []string{
		`"IdentificadorSeguro"`,
		`"Descripcion"`,
		`"Poliza"`,
		`"NombreCompania"`,
		`"CodigoCompania"`,
		`"CodigoTipoSeguro"`,
		`"CorrelativoPoliza"`,
		`"Tasa"`,
		`"Factor"`,
		`"IndicadorPolizaIndividual"`,
		`"PorValorCuota"`,
		`"IndicadorPolizaExterna"`,
	}

	lastIdx := -1
	jsonStr := string(data)
	for _, field := range fieldOrder {
		idx := strings.Index(jsonStr, field)
		if idx == -1 {
			t.Fatalf("field %s not found in JSON", field)
		}
		if idx <= lastIdx {
			t.Fatalf("field %s out of order: idx %d <= lastIdx %d", field, idx, lastIdx)
		}
		lastIdx = idx
	}

	// Test null preservation on all nullable fields
	itemNull := SeguroItem{
		IdentificadorSeguro: "ID-123",
		Descripcion:         "Seguro Null",
		Poliza:              "POL-1",
		NombreCompania:      "Compania",
	}

	dataNull, err := json.Marshal(itemNull)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedNull := `{"IdentificadorSeguro":"ID-123","Descripcion":"Seguro Null","Poliza":"POL-1","NombreCompania":"Compania","CodigoCompania":null,"CodigoTipoSeguro":null,"CorrelativoPoliza":null,"Tasa":null,"Factor":null,"IndicadorPolizaIndividual":null,"PorValorCuota":null,"IndicadorPolizaExterna":null}`
	if string(dataNull) != expectedNull {
		t.Errorf("\ngot:  %s\nwant: %s", string(dataNull), expectedNull)
	}
}

func TestNoResultsResponseSerialization(t *testing.T) {
	resp := NewNoResultsResponse()

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("unexpected error marshaling NoResultsResponse: %v", err)
	}

	expected := `{"codRespuesta":3,"Mensaje":"Sin resultados.","Excepcion":"Ninguna"}`
	if string(data) != expected {
		t.Errorf("got %s, want %s", string(data), expected)
	}

	// Verify exact order: codRespuesta, Mensaje, Excepcion
	idxCod := strings.Index(string(data), `"codRespuesta"`)
	idxMen := strings.Index(string(data), `"Mensaje"`)
	idxExc := strings.Index(string(data), `"Excepcion"`)

	if !(idxCod < idxMen && idxMen < idxExc) {
		t.Errorf("fields out of order in NoResultsResponse: cod=%d, men=%d, exc=%d", idxCod, idxMen, idxExc)
	}
}
