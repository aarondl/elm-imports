package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestString(t *testing.T) {
	t.Parallel()

	i := importDef{
		Name:  "Api.Data",
		Alias: "AD",
		Exposing: []importSymbol{
			{Name: "Document", Constructors: []string{".."}},
			{Name: "Route"},
			{Name: "Data", Constructors: []string{"Abc", "Def"}},
		},
	}

	assert.Equal(t,
		"import Api.Data as AD exposing (Document(..), Route, Data(Abc, Def))",
		i.String())
}

func TestParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		In   string
		Want importDef
	}{
		{
			In: "import Browser",
			Want: importDef{
				Name: "Browser",
			},
		},
		{
			In: "import Browser as Nav.Fun",
			Want: importDef{
				Name:  "Browser",
				Alias: "Nav.Fun",
			},
		},
		{
			In: "import Browser exposing (Document(Abc, X), NavKey(..), Other)",
			Want: importDef{
				Name: "Browser",
				Exposing: []importSymbol{
					{Name: "Document", Constructors: []string{"Abc", "X"}},
					{Name: "NavKey", Constructors: []string{".."}},
					{Name: "Other"}},
			},
		},
		{
			In: "import Platform.Cmd as Cmd exposing (Cmd)",
			Want: importDef{
				Name:  "Platform.Cmd",
				Alias: "Cmd",
				Exposing: []importSymbol{
					{Name: "Cmd"},
				},
			},
		},
	}

	for i, test := range tests {
		def, err := parseImportDef(test.In)
		if err != nil {
			t.Error(err)
		}

		assert.Equal(t, test.Want, def, "test %d", i)
	}
}
