package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vshulcz/deja-vu/internal/inspect"
	"github.com/vshulcz/deja-vu/internal/usage"
)

func TestMCPInspectMode(t *testing.T) {
	tmpDir := t.TempDir()
	indexDir := filepath.Join(tmpDir, "index")
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		t.Fatal(err)
	}

	dshRoot := filepath.Join(tmpDir, "dsh-sessions")
	sessionDir := filepath.Join(dshRoot, "test-proj", "session-test-mcp-001")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEJA_DEEPSEEK_ROOT", dshRoot)

	logPath := filepath.Join(sessionDir, "session.v3.jsonl")
	header := `{"type":"session","version":3,"id":"test-mcp-001","createdAt":1727612345000,"cwd":"/workspace/test-proj"}` + "\n"
	events := `{"type":"user/message","seq":1,"time":1727612345100,"data":{"content":"hello","source":{"kind":"user"}}}` + "\n" +
		`{"type":"tool/call","seq":2,"time":1727612345200,"data":{"callId":"c1","name":"bash","arguments":{"command":"pwd"}}}` + "\n" +
		`{"type":"tool/result","seq":3,"time":1727612345300,"data":{"callId":"c1","content":"/workspace/test-proj\n"}}` + "\n" +
		`{"type":"turn/end","seq":4,"time":1727612345400,"data":{"turn":0,"reason":"completed"}}` + "\n"
	if err := os.WriteFile(logPath, []byte(header+events), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Call via MCP with target in args
	raw := json.RawMessage(`{"mode":"inspect","target":"deepseek:test-mcp-001","view":"session"}`)
	out, err := callMCPTool(indexDir, "deja", raw)
	if err != nil {
		t.Fatalf("callMCPTool failed: %v", err)
	}

	var resp inspect.Response
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("unmarshal MCP inspect response: %v\nraw: %s", err, out)
	}
	if resp.SchemaVersion != 2 {
		t.Errorf("schema_version = %d, want 2", resp.SchemaVersion)
	}
	if resp.Completeness != inspect.CompletenessComplete {
		t.Errorf("completeness = %q, want %q", resp.Completeness, inspect.CompletenessComplete)
	}
	if resp.Session == nil || resp.Session.ID != "test-mcp-001" {
		t.Errorf("session ID mismatch: %v", resp.Session)
	}
	if !strings.Contains(resp.Notice, "untrusted reference data") {
		t.Errorf("notice missing untrusted framing: %q", resp.Notice)
	}

	// 2. Call via MCP with q shorthand
	rawQ := json.RawMessage(`{"mode":"inspect","q":"deepseek:test-mcp-001","view":"timeline"}`)
	outQ, err := callMCPTool(indexDir, "deja", rawQ)
	if err != nil {
		t.Fatalf("callMCPTool with q failed: %v", err)
	}
	var respTL inspect.Response
	if err := json.Unmarshal([]byte(outQ), &respTL); err != nil {
		t.Fatalf("unmarshal timeline response: %v", err)
	}
	if respTL.View != "timeline" || respTL.Timeline == nil {
		t.Fatalf("expected timeline view: %v", respTL)
	}
	if len(respTL.Timeline.Items) != 4 {
		t.Errorf("expected 4 timeline items, got %d", len(respTL.Timeline.Items))
	}

	// 3. Verify usage accounting recorded the inspect event
	eventsRecorded := usage.Events(indexDir, 10)
	var foundInspectUsage bool
	for _, ev := range eventsRecorded {
		if ev.Kind == usage.KindInspect {
			foundInspectUsage = true
			break
		}
	}
	if !foundInspectUsage {
		t.Error("expected usage.KindInspect event in usage log")
	}

	// 4. Unsupported harness via MCP returns unsupported without error
	rawUnsupp := json.RawMessage(`{"mode":"inspect","target":"claude:some-session"}`)
	outUnsupp, err := callMCPTool(indexDir, "deja", rawUnsupp)
	if err != nil {
		t.Fatalf("unexpected error on unsupported harness: %v", err)
	}
	var respUnsupp inspect.Response
	if err := json.Unmarshal([]byte(outUnsupp), &respUnsupp); err != nil {
		t.Fatalf("unmarshal unsupported response: %v", err)
	}
	if respUnsupp.Completeness != inspect.CompletenessUnsupported {
		t.Errorf("completeness = %q, want unsupported", respUnsupp.Completeness)
	}

	// 5. Empty target returns error
	rawEmpty := json.RawMessage(`{"mode":"inspect"}`)
	_, errEmpty := callMCPTool(indexDir, "deja", rawEmpty)
	if errEmpty == nil {
		t.Fatal("expected error for empty target")
	}
}
