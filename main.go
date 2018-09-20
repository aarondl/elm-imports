package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
)

var (
	flagOutput string
)

func main() {
	flag.StringVar(&flagOutput, "output", "", "Write to this file instead of displaying diffs")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "must supply a file argument")
		os.Exit(1)
	}

	input, err := os.Open(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	output := &bytes.Buffer{}

	err = rewriteElmImports(input, output)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err = input.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var outputWriter io.Writer = os.Stdout
	if len(flagOutput) != 0 {
		outputWriter, err = os.Create(flagOutput)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	if _, err = io.Copy(outputWriter, output); err != nil {
		fmt.Fprintln(os.Stderr, "failed to copy output to output file", err)
		os.Exit(1)
	}

	if closer, ok := outputWriter.(io.Closer); ok {
		if err = closer.Close(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
