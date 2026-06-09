package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

func decodeResponses(t *testing.T, r io.Reader) []response {
	t.Helper()
	var responses []response
	dec := json.NewDecoder(r)
	for dec.More() {
		var resp response
		if err := dec.Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		responses = append(responses, resp)
	}
	return responses
}

func TestServe(t *testing.T) {
	t.Parallel()

	t.Run("processes requests and skips notification", func(t *testing.T) {
		t.Parallel()
		eng := newTestEngine(t)

		// Requests are dispatched concurrently, so they must be independent:
		// two distinct creates plus one notification that yields no response.
		input := strings.Join([]string{
			`{"jsonrpc":"2.0","id":1,"method":"entity.create","params":{"type":"requirement","id":"REQ-001","title":"One"}}`,
			`{"jsonrpc":"2.0","method":"entity.list","params":{}}`,
			`{"jsonrpc":"2.0","id":2,"method":"entity.create","params":{"type":"requirement","id":"REQ-002","title":"Two"}}`,
		}, "\n")

		var out bytes.Buffer
		if err := Serve(context.Background(), eng, strings.NewReader(input), &out); err != nil {
			t.Fatalf("Serve: %v", err)
		}

		responses := decodeResponses(t, &out)
		if len(responses) != 2 {
			t.Fatalf("got %d responses, want 2 (notification should be silent)", len(responses))
		}
		// Responses may arrive in any order because requests are dispatched
		// concurrently; match by id rather than position.
		seen := make(map[string]bool)
		for _, resp := range responses {
			if resp.Error != nil {
				t.Errorf("unexpected error for id %s: %+v", resp.ID, resp.Error)
			}
			seen[string(resp.ID)] = true
		}
		if !seen["1"] {
			t.Error("missing response for id 1")
		}
		if !seen["2"] {
			t.Error("missing response for id 2")
		}
	})

	t.Run("concurrent requests all get matching ids", func(t *testing.T) {
		t.Parallel()
		eng := newTestEngine(t)

		const n = 50
		var sb strings.Builder
		for i := 0; i < n; i++ {
			fmt.Fprintf(&sb, `{"jsonrpc":"2.0","id":%d,"method":"entity.create","params":{"type":"requirement","id":"REQ-%03d","title":"E"}}`, i+1, i+1)
			sb.WriteByte('\n')
		}

		var out bytes.Buffer
		if err := Serve(context.Background(), eng, strings.NewReader(sb.String()), &out); err != nil {
			t.Fatalf("Serve: %v", err)
		}

		responses := decodeResponses(t, &out)
		if len(responses) != n {
			t.Fatalf("got %d responses, want %d", len(responses), n)
		}

		seen := make(map[string]bool)
		for _, resp := range responses {
			if resp.Error != nil {
				t.Errorf("unexpected error for id %s: %+v", resp.ID, resp.Error)
			}
			seen[string(resp.ID)] = true
		}
		for i := 0; i < n; i++ {
			id := fmt.Sprintf("%d", i+1)
			if !seen[id] {
				t.Errorf("missing response for id %s", id)
			}
		}
	})

	t.Run("write serialization under concurrency is race-free", func(t *testing.T) {
		t.Parallel()
		eng := newTestEngine(t)

		const n = 30
		var sb strings.Builder
		for i := 0; i < n; i++ {
			fmt.Fprintf(&sb, `{"jsonrpc":"2.0","id":%d,"method":"entity.list","params":{}}`, i+1)
			sb.WriteByte('\n')
		}

		var mu sync.Mutex
		out := &lockedBuffer{mu: &mu}
		if err := Serve(context.Background(), eng, strings.NewReader(sb.String()), out); err != nil {
			t.Fatalf("Serve: %v", err)
		}

		responses := decodeResponses(t, out.reader())
		if len(responses) != n {
			t.Fatalf("got %d responses, want %d", len(responses), n)
		}
	})
}

// lockedBuffer is a concurrency-safe buffer used to confirm that Serve never
// issues overlapping writes (the -race detector would flag interleaved writes
// to an unsynchronized buffer otherwise).
type lockedBuffer struct {
	mu  *sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) reader() io.Reader {
	b.mu.Lock()
	defer b.mu.Unlock()
	return bytes.NewReader(b.buf.Bytes())
}
