package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/opsorch/opsorch-core/schema"
	coreservice "github.com/opsorch/opsorch-core/service"
	"github.com/opsorch/opsorch-pagerduty-adapter/service"
)

var provider coreservice.Provider

func main() {
	run(os.Stdin, os.Stdout)
}

func run(r io.Reader, w io.Writer) {
	scanner := bufio.NewScanner(r)
	enc := json.NewEncoder(w)

	for scanner.Scan() {
		var req struct {
			Method  string          `json:"method"`
			Config  map[string]any  `json:"config"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			writeError(enc, fmt.Sprintf("parse request: %v", err))
			continue
		}

		prov, err := ensureProvider(req.Config)
		if err != nil {
			writeError(enc, fmt.Sprintf("init provider: %v", err))
			continue
		}

		ctx := context.Background()
		switch req.Method {
		case "service.query":
			var q schema.ServiceQuery
			if len(req.Payload) > 0 {
				if err := json.Unmarshal(req.Payload, &q); err != nil {
					writeError(enc, fmt.Sprintf("decode query: %v", err))
					continue
				}
			}
			services, err := prov.Query(ctx, q)
			if err != nil {
				writeError(enc, err.Error())
				continue
			}
			writeResult(enc, services)

		default:
			writeError(enc, fmt.Sprintf("unknown method: %s", req.Method))
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		writeError(enc, fmt.Sprintf("scanner error: %v", err))
	}
}

func ensureProvider(cfg map[string]any) (coreservice.Provider, error) {
	if provider != nil {
		return provider, nil
	}
	prov, err := service.New(cfg)
	if err != nil {
		return nil, err
	}
	provider = prov
	return provider, nil
}

func writeResult(enc *json.Encoder, v any) {
	enc.Encode(map[string]any{"result": v})
}

func writeError(enc *json.Encoder, msg string) {
	enc.Encode(map[string]any{"error": msg})
}
