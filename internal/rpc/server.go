package rpc

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"sync"

	"github.com/tyeongkim/spec-graph/pkg/specgraph"
)

// Serve owns in until it returns, closes it on shutdown, and waits for all
// dispatched work. The reader's Close must unblock an outstanding Read.
func Serve(ctx context.Context, engine *specgraph.Engine, in io.ReadCloser, out io.Writer) error {
	serving, cancel := context.WithCancel(ctx)
	defer cancel()
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-serving.Done()
		_ = in.Close()
	}()
	dispatcher := NewDispatcher(engine)
	var writeMu sync.Mutex
	var writeErr error
	var wg sync.WaitGroup
	writeResponse := func(b []byte) {
		writeMu.Lock()
		defer writeMu.Unlock()
		if writeErr != nil {
			return
		}
		b = append(b, '\n')
		n, err := out.Write(b)
		if err == nil && n != len(b) {
			err = io.ErrShortWrite
		}
		if err != nil {
			writeErr = err
			cancel()
		}
	}
	dec := json.NewDecoder(bufio.NewReader(in))
	for serving.Err() == nil {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			if serving.Err() == nil && err != io.EOF {
				writeResponse(dispatcher.marshalResponse(errorResponse(nil, codeParseError, "parse error", nil)))
			}
			break
		}
		writeMu.Lock()
		if serving.Err() != nil {
			writeMu.Unlock()
			break
		}
		wg.Add(1)
		go func(message json.RawMessage) {
			defer wg.Done()
			respBytes, isNotification := dispatcher.Handle(serving, message)
			if !isNotification && respBytes != nil {
				writeResponse(respBytes)
			}
		}(raw)
		writeMu.Unlock()
	}
	wg.Wait()
	cancel()
	<-stopped
	if writeErr != nil {
		return writeErr
	}
	return ctx.Err()
}
