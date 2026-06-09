package rpc

import (
	"context"
	"encoding/json"
	"testing"
)

func TestDispatcherHandle(t *testing.T) {
	t.Parallel()

	t.Run("create then get roundtrip", func(t *testing.T) {
		t.Parallel()
		d := NewDispatcher(newTestEngine(t))
		ctx := context.Background()

		createReq := `{"jsonrpc":"2.0","id":1,"method":"entity.create","params":{"type":"requirement","id":"REQ-001","title":"Test"}}`
		resp, isNotif := d.Handle(ctx, []byte(createReq))
		if isNotif {
			t.Fatal("create should not be a notification")
		}
		var createEnv response
		if err := json.Unmarshal(resp, &createEnv); err != nil {
			t.Fatalf("unmarshal create response: %v", err)
		}
		if createEnv.Error != nil {
			t.Fatalf("create returned error: %+v", createEnv.Error)
		}

		getReq := `{"jsonrpc":"2.0","id":2,"method":"entity.get","params":{"id":"REQ-001"}}`
		resp, _ = d.Handle(ctx, []byte(getReq))
		var getEnv response
		if err := json.Unmarshal(resp, &getEnv); err != nil {
			t.Fatalf("unmarshal get response: %v", err)
		}
		if getEnv.Error != nil {
			t.Fatalf("get returned error: %+v", getEnv.Error)
		}
		var result struct {
			Entity struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"entity"`
		}
		if err := json.Unmarshal(getEnv.Result, &result); err != nil {
			t.Fatalf("unmarshal get result: %v", err)
		}
		if result.Entity.ID != "REQ-001" {
			t.Errorf("entity id = %q, want REQ-001", result.Entity.ID)
		}
		if result.Entity.Title != "Test" {
			t.Errorf("entity title = %q, want Test", result.Entity.Title)
		}
	})

	t.Run("echoes id types", func(t *testing.T) {
		t.Parallel()
		d := NewDispatcher(newTestEngine(t))
		ctx := context.Background()

		stringIDReq := `{"jsonrpc":"2.0","id":"abc","method":"entity.list","params":{}}`
		resp, _ := d.Handle(ctx, []byte(stringIDReq))
		var env response
		if err := json.Unmarshal(resp, &env); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if string(env.ID) != `"abc"` {
			t.Errorf("id = %s, want \"abc\"", env.ID)
		}
	})

	t.Run("method not found", func(t *testing.T) {
		t.Parallel()
		d := NewDispatcher(newTestEngine(t))
		ctx := context.Background()

		req := `{"jsonrpc":"2.0","id":1,"method":"does.not.exist","params":{}}`
		resp, _ := d.Handle(ctx, []byte(req))
		var env response
		if err := json.Unmarshal(resp, &env); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if env.Error == nil {
			t.Fatal("expected error, got nil")
		}
		if env.Error.Code != codeMethodNotFound {
			t.Errorf("code = %d, want %d", env.Error.Code, codeMethodNotFound)
		}
	})

	t.Run("parse error", func(t *testing.T) {
		t.Parallel()
		d := NewDispatcher(newTestEngine(t))
		ctx := context.Background()

		resp, isNotif := d.Handle(ctx, []byte(`{not valid json`))
		if isNotif {
			t.Fatal("parse error should not be a notification")
		}
		var env response
		if err := json.Unmarshal(resp, &env); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if env.Error == nil || env.Error.Code != codeParseError {
			t.Errorf("expected parse error code %d, got %+v", codeParseError, env.Error)
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		t.Parallel()
		d := NewDispatcher(newTestEngine(t))
		ctx := context.Background()

		req := `{"jsonrpc":"2.0","id":1,"method":"entity.get","params":{"id":12345}}`
		resp, _ := d.Handle(ctx, []byte(req))
		var env response
		if err := json.Unmarshal(resp, &env); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if env.Error == nil {
			t.Fatal("expected error, got nil")
		}
		if env.Error.Code != codeInvalidParams {
			t.Errorf("code = %d, want %d", env.Error.Code, codeInvalidParams)
		}
	})

	t.Run("notification yields no response", func(t *testing.T) {
		t.Parallel()
		d := NewDispatcher(newTestEngine(t))
		ctx := context.Background()

		req := `{"jsonrpc":"2.0","method":"entity.list","params":{}}`
		resp, isNotif := d.Handle(ctx, []byte(req))
		if !isNotif {
			t.Error("expected notification")
		}
		if resp != nil {
			t.Errorf("expected nil response bytes, got %s", resp)
		}
	})

	t.Run("engine not-found maps to invalid params with data", func(t *testing.T) {
		t.Parallel()
		d := NewDispatcher(newTestEngine(t))
		ctx := context.Background()

		req := `{"jsonrpc":"2.0","id":1,"method":"entity.get","params":{"id":"REQ-999"}}`
		resp, _ := d.Handle(ctx, []byte(req))
		var env response
		if err := json.Unmarshal(resp, &env); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if env.Error == nil {
			t.Fatal("expected error, got nil")
		}
		if env.Error.Code != codeInvalidParams {
			t.Errorf("code = %d, want %d", env.Error.Code, codeInvalidParams)
		}
		dataBytes, _ := json.Marshal(env.Error.Data)
		var data errorData
		if err := json.Unmarshal(dataBytes, &data); err != nil {
			t.Fatalf("unmarshal error data: %v", err)
		}
		if data.Code != "not_found" {
			t.Errorf("data code = %q, want not_found", data.Code)
		}
	})

	t.Run("invalid request missing version", func(t *testing.T) {
		t.Parallel()
		d := NewDispatcher(newTestEngine(t))
		ctx := context.Background()

		req := `{"id":1,"method":"entity.list","params":{}}`
		resp, _ := d.Handle(ctx, []byte(req))
		var env response
		if err := json.Unmarshal(resp, &env); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if env.Error == nil || env.Error.Code != codeInvalidRequest {
			t.Errorf("expected invalid request code %d, got %+v", codeInvalidRequest, env.Error)
		}
	})
}
