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

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
)

var rgxSymbols = regexp.MustCompile(`(?:[\(\s])[A-Z][A-Za-z0-9]*(?:\.[A-Za-z]+)+`)

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

		symbs := rgxSymbols.FindAllString(line, -1)
		for _, s := range symbs {
			// Remove the ( or whitespace at the beginning of the symbol regex
			ef.Symbols = append(ef.Symbols, s[1:])
		}
		// ef.Symbols = append(ef.Symbols, ...)
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

const (
	stateNone = iota
	stateModule
	stateImports
	stateSymbols
	stateComments
	stateCommentLine
)

func rewriteElmImports(in io.Reader, out io.Writer) error {
	ef := &elmFile{}

	bufferLines := make([][]byte, 0, 512)
	scanner := bufio.NewScanner(in)
	writer := bufio.NewWriter(out)

	lineIndex := 0
	startImports := -1
	state := stateNone
	prevState := stateNone

	for ; scanner.Scan(); lineIndex++ {
		line := scanner.Bytes()

		if flagDebug {
			fmt.Fprintf(os.Stderr, "L(%d", state)
		}

		commentIndex := bytes.Index(line, []byte{'-', '-'})
		if state != stateComments && bytes.HasPrefix(line, []byte{'{', '-'}) && bytes.HasSuffix(line, []byte{'-', '}'}) {
			if state != stateCommentLine {
				prevState = state
			}
			state = stateCommentLine
		} else if state != stateComments && bytes.HasPrefix(line, []byte{'{', '-'}) {
			prevState = state
			state = stateComments
		} else if commentIndex == 0 {
			if state != stateCommentLine {
				prevState = state
			}
			state = stateCommentLine
		} else if state == stateCommentLine {
			state = prevState
		} else {
			switch state {
			case stateNone:
				if bytes.HasPrefix(line, []byte("module")) || bytes.HasPrefix(line, []byte("port module")) {
					state = stateModule
				} else if bytes.HasPrefix(line, []byte("import")) {
					state = stateImports
					startImports = lineIndex
				} else if len(line) != 0 {
					state = stateSymbols
					startImports = lineIndex
				}
			case stateModule:
				if bytes.HasPrefix(line, []byte("import")) {
					state = stateImports
					startImports = lineIndex
				} else if len(line) == 0 {
					state = stateNone
				}
			case stateImports:
				if len(line) != 0 && !bytes.HasPrefix(line, []byte("import")) {
					state = stateSymbols
				}
			}
		}

		if flagDebug {
			fmt.Fprintf(os.Stderr, "->%d): %s\n", state, line)
		}

		switch state {
		case stateComments:
			if bytes.HasPrefix(line, []byte{'-', '}'}) {
				state = prevState
			}
		case stateImports:
			// Ignore whitespace between imports
			if len(line) == 0 {
				continue
			}

			def, err := parseImportDef(string(line))
			if err != nil {
				return errors.Wrapf(err, "failed to parse line: %s", line)
			}
			ef.Imports = append(ef.Imports, def)
			continue
		case stateSymbols:
			// Read symbols if this line isn't a comment, remove the parts
			// of it that are a comment
			if commentIndex != 0 {
				withoutComment := line
				if commentIndex >= 0 {
					withoutComment = line[:commentIndex]
				}

				for _, sym := range rgxSymbols.FindAll(withoutComment, -1) {
					// Remove the ( or the whitespace at the beginning
					sym = sym[1:]
					ef.Symbols = append(ef.Symbols, string(sym))
				}
			}
		}

		lineCopy := make([]byte, len(line))
		copy(lineCopy, line)
		bufferLines = append(bufferLines, lineCopy)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if flagDebug {
		sort.Strings(ef.Symbols)
		fmt.Fprintf(os.Stderr, "Before: %s\n", spew.Sdump(ef))
	}

	ef.addMissing()
	ef.remove019Defaults()
	ef.removeUnused()
	ef.sortImports()

	if flagDebug {
		fmt.Fprintf(os.Stderr, "After: %s\n", spew.Sdump(ef.Imports))
	}

	if flagDebug {
		fmt.Fprintf(os.Stderr, "Output Line: %d\n", startImports)
	}

	if startImports < 0 {
		writeBufferedLines(writer, bufferLines)
		return writer.Flush()
	}

	// Buffered error handling allows us to omit error handling
	// and the error will be returned on flush.
	writeBufferedLines(writer, bufferLines[:startImports])
	for _, imp := range ef.Imports {
		_, _ = fmt.Fprintf(writer, "%s\n", imp)
	}
	_, _ = writer.Write([]byte{'\n'})
	writeBufferedLines(writer, bufferLines[startImports:])

	return writer.Flush()
}

func writeBufferedLines(out io.Writer, buffer [][]byte) {
	for _, buf := range buffer {
		_, _ = out.Write(buf)
		_, _ = out.Write([]byte{'\n'})
	}
}
