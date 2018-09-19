package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortImports(t *testing.T) {
	t.Parallel()

	ef := &elmFile{
		Imports: []importDef{
			{Name: "List", Exposing: []importSymbol{
				{Name: "Def", Constructors: []string{".."}},
				{Name: "Abc", Constructors: []string{"Deny", "Allow"}},
			}},
			{Name: "Basics"},
		},
	}

	ef.sortImports()

	assert.Equal(t, importDef{Name: "Basics"}, ef.Imports[0])
	assert.Equal(t, importDef{Name: "List", Exposing: []importSymbol{
		{Name: "Abc", Constructors: []string{"Allow", "Deny"}},
		{Name: "Def", Constructors: []string{".."}},
	}}, ef.Imports[1])
}

func TestRemoveUnused(t *testing.T) {
	t.Parallel()

	ef, err := parseElmFile("testdata/Main.elm")
	if err != nil {
		t.Fatal(err)
	}

	ef.removeUnused()
	for _, imp := range ef.Imports {
		if imp.Name == "Browser" {
			t.Error("browser should be gone")
		}
	}
}

func TestAddMissing(t *testing.T) {
	t.Parallel()

	ef := &elmFile{
		Symbols: []string{"Route.Happiness", "Api.Data.sad"},
	}

	ef.addMissing()

	ef.sortImports()

	assert.Equal(t, importDef{Name: "Api.Data"}, ef.Imports[0])
	assert.Equal(t, importDef{Name: "Route"}, ef.Imports[1])
}

func TestRemoveDefaults(t *testing.T) {
	t.Parallel()
	ef := &elmFile{
		Imports: []importDef{
			{Name: "Basics", Exposing: []importSymbol{{Name: ".."}}},
			{Name: "List", Exposing: []importSymbol{{Name: "List"}, {Name: "(::)"}}},
			{Name: "Maybe", Exposing: []importSymbol{{Name: "Maybe", Constructors: []string{".."}}}},
			{Name: "Result", Exposing: []importSymbol{{Name: "Result", Constructors: []string{".."}}}},
			{Name: "String", Exposing: []importSymbol{{Name: "String"}}},
			{Name: "Char", Exposing: []importSymbol{{Name: "Char"}}},
			{Name: "Tuple"},
			{Name: "Debug"},
			{Name: "Platform", Exposing: []importSymbol{{Name: "Program"}}},
			{Name: "Platform.Cmd", Alias: "Cmd", Exposing: []importSymbol{{Name: "Cmd"}}},
			{Name: "Platform.Sub", Alias: "Sub", Exposing: []importSymbol{{Name: "Sub"}}},
		},
	}

	ef.remove019Defaults()

	assert.Len(t, ef.Imports, 0)
}

func TestRequiredModules(t *testing.T) {
	t.Parallel()

	ef, err := parseElmFile("testdata/Missing.elm")
	if err != nil {
		t.Fatal(err)
	}

	mods := ef.requiredModules()
	assert.Len(t, mods, 2)
	assert.Contains(t, mods, "Api.Data")
	assert.Contains(t, mods, "Api.Auth")
}

func TestParseElmFile(t *testing.T) {
	t.Parallel()

	ef, err := parseElmFile("testdata/Main.elm")
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, ef.Imports, 5)
	assert.Equal(t, importDef{Name: "Browser"}, ef.Imports[0])
	assert.Equal(t, importDef{Name: "Browser.Navigation", Alias: "Nav"}, ef.Imports[1])
	assert.Equal(t, importDef{Name: "Html", Exposing: []importSymbol{{Name: ".."}}}, ef.Imports[2])
	assert.Equal(t, importDef{Name: "Html.Attributes", Exposing: []importSymbol{{Name: ".."}}}, ef.Imports[3])
	assert.Equal(t, importDef{Name: "Routes", Exposing: []importSymbol{{Name: "Route"}}}, ef.Imports[4])

	assert.Len(t, ef.Symbols, 2)
	assert.Equal(t, []string{"Nav.Key", "Nav.makeKey"}, ef.Symbols)

	assert.Equal(t, ef.FirstImport, 3)
	assert.Equal(t, ef.LastImport, 7)
}
