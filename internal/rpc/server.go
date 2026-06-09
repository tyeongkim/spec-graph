package rpc

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"sync"

	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

// Serve reads JSON-RPC 2.0 messages from in and writes responses to out until
// in is exhausted (e.g. stdin closes) or ctx is cancelled. Messages are decoded
// sequentially by a single reader, but each request is dispatched to its own
// goroutine so a slow request does not block others. Writes to out are
// serialized through a mutex because out (typically stdout) is not safe for
// concurrent writes. Serve blocks until all in-flight requests have completed.
//
// Data-race safety for the underlying graph is provided by the engine's own
// RWMutex; Serve only adds write serialization on the output stream.
func Serve(ctx context.Context, engine *specgraph.Engine, in io.Reader, out io.Writer) error {
	dispatcher := NewDispatcher(engine)

	var writeMu sync.Mutex
	var wg sync.WaitGroup

	writeResponse := func(b []byte) {
		writeMu.Lock()
		defer writeMu.Unlock()
		_, _ = out.Write(b)
		_, _ = out.Write([]byte("\n"))
	}

	dec := json.NewDecoder(bufio.NewReader(in))
	for {
		if err := ctx.Err(); err != nil {
			break
		}

		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			// A decode error here means the stream is malformed at the framing
			// level; emit a parse error and stop reading, since the decoder
			// cannot reliably resynchronize.
			writeResponse(dispatcher.marshalResponse(errorResponse(nil, codeParseError, "parse error", nil)))
			break
		}

		wg.Add(1)
		go func(message json.RawMessage) {
			defer wg.Done()
			respBytes, isNotification := dispatcher.Handle(ctx, message)
			if isNotification || respBytes == nil {
				return
			}
			writeResponse(respBytes)
		}(raw)
	}

	wg.Wait()
	return nil
}
