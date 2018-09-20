package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

var rgxSymbols = regexp.MustCompile(`\b[A-Z][A-Za-z0-9]*(?:\.[A-Za-z]+)+`)

type elmFile struct {
	FirstImport int
	LastImport  int
	Imports     []importDef
	Symbols     []string
}

func (e *elmFile) sortImports() {
	sort.Slice(e.Imports, func(i, j int) bool {
		return e.Imports[i].Name < e.Imports[j].Name
	})

	for _, imp := range e.Imports {
		sort.Slice(imp.Exposing, func(i, j int) bool {
			return imp.Exposing[i].Name < imp.Exposing[j].Name
		})

		for _, exp := range imp.Exposing {
			if len(exp.Constructors) > 1 {
				sort.Strings(exp.Constructors)
			}
		}
	}
}

func (e *elmFile) requiredModules() map[string]struct{} {
	modules := make(map[string]struct{})
	for _, sym := range e.Symbols {
		lastDot := strings.LastIndexByte(sym, '.')
		module := sym[:lastDot]
		modules[module] = struct{}{}
	}

	return modules
}

func (e *elmFile) addMissing() {
	required := e.requiredModules()

	search := func(search string, space []importDef) bool {
		for _, imp := range space {
			name := imp.Alias
			if len(name) == 0 {
				name = imp.Name
			}

			if search == name {
				return true
			}
		}

		return false
	}

	for r := range required {
		found := search(r, e.Imports) || search(r, defaultImports)
		if !found {
			e.Imports = append(e.Imports, importDef{Name: r})
		}
	}
}

func (e *elmFile) removeUnused() {
	required := e.requiredModules()

	for i := 0; i < len(e.Imports); i++ {
		imp := e.Imports[i]

		if imp.Exposing != nil {
			// Cannot remove this
			continue
		}

		name := imp.Alias
		if len(name) == 0 {
			name = imp.Name
		}

		_, needThis := required[name]
		if needThis {
			continue
		}

		e.Imports[i], e.Imports[len(e.Imports)-1] = e.Imports[len(e.Imports)-1], e.Imports[i]
		e.Imports = e.Imports[:len(e.Imports)-1]
		i--
	}
}

var defaultImports = []importDef{
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
}

func (e *elmFile) remove019Defaults() {
	for i := 0; i < len(e.Imports); i++ {
		imp := e.Imports[i]

		found := false
		for _, d := range defaultImports {
			if d.Equal(imp) {
				found = true
				break
			}
		}

		if !found {
			continue
		}

		e.Imports[i], e.Imports[len(e.Imports)-1] = e.Imports[len(e.Imports)-1], e.Imports[i]
		e.Imports = e.Imports[:len(e.Imports)-1]
		i--
	}
}

func parseElmFile(file string) (ef elmFile, err error) {
	f, err := os.Open(file)
	if err != nil {
		return ef, err
	}
	defer f.Close()

	ef.FirstImport = -1
	ef.LastImport = -1
	lineNumber := 1
	scanner := bufio.NewScanner(f)
	for ; scanner.Scan(); lineNumber++ {
		line := scanner.Text()

		if strings.HasPrefix(line, "module") {
			continue
		}

		if strings.HasPrefix(line, "import") {
			if ef.FirstImport < 0 {
				ef.FirstImport = lineNumber
			}
			ef.LastImport = lineNumber
			def, err := parseImportDef(line)
			if err != nil {
				return ef, err
			}
			ef.Imports = append(ef.Imports, def)
			continue
		}

		ef.Symbols = append(ef.Symbols, rgxSymbols.FindAllString(line, -1)...)
	}

	if err := scanner.Err(); err != nil {
		return ef, err
	}

	return ef, nil
}

func rewriteElmImportsFiles(in, out string) error {
	input, err := os.Open(in)
	if err != nil {
		return errors.Wrap(err, "failed to open input file")
	}

	output, err := os.Create(out)
	if err != nil {
		return errors.Wrap(err, "failed to create output file")
	}

	errs := []error{
		rewriteElmImports(input, output),
		input.Close(),
		output.Close(),
	}

	var errMsg string
	for _, e := range errs {
		if e != nil {
			if len(errMsg) != 0 {
				errMsg += ", "
			}
			errMsg += e.Error()
		}
	}

	if len(errMsg) != 0 {
		return errors.New(errMsg)
	}

	return nil
}

func rewriteElmImports(in io.Reader, out io.Writer) error {
	ef := &elmFile{}
	scanningImports := false
	scanningSymbols := false
	sawModuleLine := false
	inComment := false
	lineIsComment := false

	bufferLines := make([][]byte, 0, 512)

	scanner := bufio.NewScanner(in)
	writer := bufio.NewWriter(out)
	for scanner.Scan() {
		lineIsComment = false
		line := scanner.Bytes()

		if bytes.HasPrefix(line, []byte("import")) {
			scanningImports = true
			def, err := parseImportDef(string(line))
			if err != nil {
				return errors.Wrapf(err, "failed to parse line: %s", line)
			}
			ef.Imports = append(ef.Imports, def)
			continue
		}

		if scanningImports {
			if len(line) == 0 {
				// Don't copy over any whitespace, and keep looking for imports
				continue
			} else {
				// Write out imports
				scanningImports = false
				scanningSymbols = true
			}
		}

		if bytes.HasPrefix(line, []byte{'{', '-'}) {
			inComment = true
		} else if bytes.HasPrefix(line, []byte{'-', '}'}) {
			inComment = false
		}

		if !inComment {
			lineIsComment = bytes.HasPrefix(line, []byte{'-', '-'})
		}

		if sawModuleLine && !inComment && !lineIsComment {
			for _, sym := range rgxSymbols.FindAll(line, -1) {
				ef.Symbols = append(ef.Symbols, string(sym))
			}
		}
		sawModuleLine = sawModuleLine || bytes.HasPrefix(line, []byte("module"))

		if scanningSymbols {
			lineCopy := make([]byte, len(line))
			copy(lineCopy, line)
			bufferLines = append(bufferLines, lineCopy)
		} else {
			if _, err := fmt.Fprintf(writer, "%s\n", line); err != nil {
				return err
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// At this point we've collected all imports and symbols and the file
	// has been written up until the point where the imports started
	// now we have to rewrite the imports, write them, and write the buffered
	// chunks of the file.

	ef.addMissing()
	ef.remove019Defaults()
	ef.removeUnused()
	ef.sortImports()

	// Buffered writer saves us from errors checks here, we do them below to
	// fail fast.
	for _, imp := range ef.Imports {
		fmt.Fprintf(writer, "%s\n", imp)
	}
	writer.Write([]byte{'\n'})

	for _, buf := range bufferLines {
		if _, err := writer.Write(buf); err != nil {
			return err
		}
		if _, err := writer.Write([]byte{'\n'}); err != nil {
			return err
		}
	}

	return writer.Flush()
}
