package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/asymmetric-effort/ginger/internal/protocol/otlp"
)

func main() {
	input := flag.String("input", "", "input trace JSON file")
	output := flag.String("output", "anonymized.json", "output file")
	flag.Parse()

	if *input == "" {
		fmt.Fprintln(os.Stderr, "Usage: anonymizer --input <file.json> --output <file.json>")
		os.Exit(1)
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read: %v\n", err)
		os.Exit(1)
	}

	var td otlp.TracesData
	if err := json.Unmarshal(data, &td); err != nil {
		fmt.Fprintf(os.Stderr, "parse: %v\n", err)
		os.Exit(1)
	}

	anonymize(&td)

	out, err := json.MarshalIndent(td, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*output, out, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "anonymized trace written to %s\n", *output)
}

var sensitiveKeys = map[string]bool{
	"user.id": true, "user.email": true, "user.name": true,
	"db.statement": true, "db.query": true,
	"http.url": true, "http.target": true,
	"enduser.id": true,
}

func anonymize(td *otlp.TracesData) {
	for ri := range td.ResourceSpans {
		scrubAttributes(&td.ResourceSpans[ri].Resource.Attributes)
		for si := range td.ResourceSpans[ri].ScopeSpans {
			for spi := range td.ResourceSpans[ri].ScopeSpans[si].Spans {
				span := &td.ResourceSpans[ri].ScopeSpans[si].Spans[spi]
				scrubAttributes(&span.Attributes)
				for ei := range span.Events {
					scrubAttributes(&span.Events[ei].Attributes)
				}
			}
		}
	}
}

func scrubAttributes(attrs *otlp.Attributes) {
	items := attrs.Items()
	newAttrs := otlp.NewAttributes()
	for _, kv := range items {
		if sensitiveKeys[kv.Key] {
			newAttrs.Set(kv.Key, otlp.StringValue("[REDACTED]"))
		} else if kv.Value.Type == otlp.AnyValueTypeString {
			newAttrs.Set(kv.Key, otlp.StringValue(scrubString(kv.Key, kv.Value.Str)))
		} else {
			newAttrs.Set(kv.Key, kv.Value)
		}
	}
	*attrs = newAttrs
}

func scrubString(key, value string) string {
	// Scrub URLs — remove query string
	if strings.Contains(key, "url") || strings.Contains(key, "uri") {
		if idx := strings.Index(value, "?"); idx >= 0 {
			return value[:idx] + "?[REDACTED]"
		}
	}
	// Scrub email addresses
	if strings.Contains(value, "@") && strings.Contains(value, ".") {
		parts := strings.SplitN(value, "@", 2)
		if len(parts) == 2 {
			return parts[0][:1] + "***@example.com"
		}
	}
	// Scrub IP addresses — zero last octet
	ip := net.ParseIP(value)
	if ip != nil && ip.To4() != nil {
		ipv4 := ip.To4()
		ipv4[3] = 0
		return ipv4.String()
	}
	return value
}
