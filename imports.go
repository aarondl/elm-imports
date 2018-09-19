package main

import (
	"bytes"
	"strings"

	"github.com/pkg/errors"
)

type importDef struct {
	Name     string
	Alias    string
	Exposing []importSymbol
}

type importSymbol struct {
	Name         string
	Constructors []string
}

func (i importDef) String() string {
	var builder strings.Builder
	builder.WriteString("import ")
	builder.WriteString(i.Name)

	if len(i.Alias) != 0 {
		builder.WriteString(" as ")
		builder.WriteString(i.Alias)
	}

	if len(i.Exposing) != 0 {
		builder.WriteString(" exposing (")

		for j, typ := range i.Exposing {
			if j != 0 {
				builder.WriteString(", ")
			}
			builder.WriteString(typ.Name)

			if len(typ.Constructors) != 0 {
				builder.WriteByte('(')
				for k, cons := range typ.Constructors {
					if k != 0 {
						builder.WriteString(", ")
					}
					builder.WriteString(cons)
				}
				builder.WriteByte(')')
			}
		}

		builder.WriteByte(')')
	}

	return builder.String()
}

func (i importDef) Equal(rhs importDef) bool {
	if i.Name != rhs.Name {
		return false
	}

	if i.Alias != rhs.Alias {
		return false
	}

	if len(i.Exposing) != len(rhs.Exposing) {
		return false
	}

	for _, lhsSym := range i.Exposing {
		found := false
		for _, rhsSym := range rhs.Exposing {
			if lhsSym.Name == rhsSym.Name {
				if !stringSliceSameElements(lhsSym.Constructors, rhsSym.Constructors) {
					return false
				}
				found = true
			}

		}

		if !found {
			return false
		}
	}

	return true
}

func stringSliceSameElements(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for _, i := range a {
		found := false
		for _, j := range b {
			if i == j {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}

func parseImportDef(in string) (imp importDef, err error) {
	fields := strings.Fields(in)
	if len(fields) < 2 {
		return imp, errors.New("import statements must have at least 2 words")
	}

	if fields[0] != "import" {
		return imp, errors.New("import must start with import")
	}

	imp.Name = fields[1]

	if len(fields) == 2 {
		return imp, nil
	}

	index := 2

	if fields[index] == "as" {
		imp.Alias = fields[3]
		index += 2
	}

	if index >= len(fields) {
		return imp, nil
	}

	if fields[index] != "exposing" {
		return imp, errors.Errorf("expected exposing, found: %s", fields[index])
	}

	first := strings.IndexByte(in, '(')
	last := strings.LastIndexByte(in, ')')
	substr := in[first+1 : last+1]

	state := "symbol"
	var def importSymbol
	buf := &bytes.Buffer{}
	for _, char := range substr {
		switch char {
		case ' ':
			continue
		case ',', ')':
			if state == "symbol" {
				name := buf.String()
				// It's possible to get a 0-length name if we set it
				// already because we opened up subsymbols
				if len(name) != 0 {
					def.Name = name
				}
				buf.Reset()
				imp.Exposing = append(imp.Exposing, def)
				def = importSymbol{}
			} else if state == "subsymbol" {
				def.Constructors = append(def.Constructors, buf.String())
				buf.Reset()

				if char == ')' {
					state = "symbol"
				}
			}
		case '(':
			if state == "symbol" {
				def.Name = buf.String()
				buf.Reset()
				state = "subsymbol"
			} else {
				panic("what")
			}
		default:
			buf.WriteRune(char)
		}
	}

	return imp, nil
}
