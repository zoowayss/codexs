package main

import (
	"os"

	"codexs/internal/codexs"
)

func main() {
	os.Exit(codexs.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
