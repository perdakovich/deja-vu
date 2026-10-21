package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vshulcz/deja-vu/internal/inspect"
)

func TestInspectCLI(t *testing.T) {
	tmpDir := t.TempDir()
	dshRoot := filepath.Join(tmpDir, "dsh-sessions")
	sessionDir := filepath.Join(dshRoot, "test-proj", "session-cli-inspect-001")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEJA_DEEPSEEK_ROOT", dshRoot)

	logPath := filepath.Join(sessionDir, "session.v4.jsonl")
	header := `{"type":"session","version":4,"id":"session-cli-inspect-001","createdAt":1727612345000,"cwd":"/workspace/test-proj"}` + "\n"
	events := `{"type":"turn/start","seq":0,"time":1727612345050,"data":{"turn":1}}` + "\n" +
		`{"type":"user/message","seq":1,"time":1727612345100,"data":{"content":"hello","source":{"kind":"user"}}}` + "\n" +
		`{"type":"tool/call","seq":2,"time":1727612345200,"data":{"callId":"c1","name":"bash","arguments":{"command":"pwd"}}}` + "\n" +
		`{"type":"tool/result","seq":3,"time":1727612345300,"data":{"callId":"c1","toolCallId":"c1","content":[{"type":"text","text":"/workspace/test-proj\n"}]}}` + "\n" +
		`{"type":"turn/end","seq":4,"time":1727612345400,"data":{"turn":1,"reason":"completed"}}` + "\n"
	if err := os.WriteFile(logPath, []byte(header+events), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Inspect session view via CLI text output
	var stdout bytes.Buffer
	err := runInspect("", []string{"deepseek:cli-inspect-001"}, &stdout)
	if err != nil {
		t.Fatalf("runInspect failed: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "deepseek:cli-inspect-001") {
		t.Errorf("output missing target: %s", out)
	}
	if !strings.Contains(out, "Turns:        1") {
		t.Errorf("output missing turn count: %s", out)
	}
	if !strings.Contains(out, "Tool Calls:   1") {
		t.Errorf("output missing tool calls: %s", out)
	}

	// 2. Inspect bare target resolution
	stdout.Reset()
	err = runInspect("", []string{"cli-inspect-001"}, &stdout)
	if err != nil {
		t.Fatalf("runInspect with bare target failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "Target:       deepseek:cli-inspect-001") {
		t.Errorf("bare target not resolved to deepseek: %s", stdout.String())
	}

	// 3. Inspect JSON output
	stdout.Reset()
	err = runInspect("", []string{"cli-inspect-001", "--view", "timeline", "--json"}, &stdout)
	if err != nil {
		t.Fatalf("runInspect --json failed: %v", err)
	}
	var resp inspect.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal json inspect output: %v", err)
	}
	if resp.View != "timeline" || resp.Timeline == nil {
		t.Fatalf("expected timeline view in response: %#v", resp)
	}
	if len(resp.Timeline.Items) != 5 {
		t.Errorf("expected 5 timeline items, got %d", len(resp.Timeline.Items))
	}

	// 4. Inspect spine view
	stdout.Reset()
	err = runInspect("", []string{"cli-inspect-001", "--view", "spine"}, &stdout)
	if err != nil {
		t.Fatalf("runInspect --view spine failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "Dialogue Spine") {
		t.Errorf("expected Dialogue Spine header: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "[USER]:") {
		t.Errorf("expected [USER]: in spine: %s", stdout.String())
	}

	// 5. Inspect state view
	stdout.Reset()
	err = runInspect("", []string{"cli-inspect-001", "--view", "state"}, &stdout)
	if err != nil {
		t.Fatalf("runInspect --view state failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "Session State") {
		t.Errorf("expected Session State header: %s", stdout.String())
	}

	// 6. Help output
	stdout.Reset()
	err = runInspect("", []string{"--help"}, &stdout)
	if err != nil {
		t.Fatalf("runInspect --help failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "Usage: deja inspect") {
		t.Errorf("help missing usage: %s", stdout.String())
	}
}
