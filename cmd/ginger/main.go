package main

import (
	"fmt"
	"os"

	"github.com/asymmetric-effort/ginger/internal/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(version.String())
		return
	}
	fmt.Fprintln(os.Stderr, "ginger: OpenTelemetry tracing toolkit for Kubernetes")
	fmt.Fprintln(os.Stderr, "Usage: ginger [command]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  version    Print version information")
	os.Exit(1)
}
