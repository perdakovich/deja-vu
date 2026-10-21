package inspect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vshulcz/deja-vu/internal/policy"
)

func TestParseTarget(t *testing.T) {
	cases := []struct {
		raw         string
		wantHarness string
		wantID      string
		wantErr     bool
	}{
		{
			raw:         "deja://session/deepseek:0de72a25-8caf-42e8-bb40-8ed04ff1c3ee",
			wantHarness: "deepseek",
			wantID:      "0de72a25-8caf-42e8-bb40-8ed04ff1c3ee",
		},
		{
			raw:         "deja://session/claude:session-xyz",
			wantHarness: "claude",
			wantID:      "session-xyz",
		},
		{
			raw:         "deepseek:session-12345",
			wantHarness: "deepseek",
			wantID:      "session-12345",
		},
		{
			raw:         "0de72a25-8caf-42e8-bb40-8ed04ff1c3ee",
			wantHarness: "",
			wantID:      "0de72a25-8caf-42e8-bb40-8ed04ff1c3ee",
		},
		{
			raw:     "   ",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		got, err := parseTarget(tc.raw)
		if (err != nil) != tc.wantErr {
			t.Fatalf("parseTarget(%q) err = %v, wantErr %v", tc.raw, err, tc.wantErr)
		}
		if tc.wantErr {
			continue
		}
		if got.Harness != tc.wantHarness {
			t.Errorf("parseTarget(%q).Harness = %q, want %q", tc.raw, got.Harness, tc.wantHarness)
		}
		if got.ID != tc.wantID {
			t.Errorf("parseTarget(%q).ID = %q, want %q", tc.raw, got.ID, tc.wantID)
		}
	}
}

func TestUnsupportedHarness(t *testing.T) {
	resp, err := Run("", Options{
		Target: "claude:session-abc",
		View:   "session",
	})
	if err != nil {
		t.Fatalf("Run unexpectedly failed: %v", err)
	}
	if resp.Completeness != CompletenessUnsupported {
		t.Fatalf("expected completeness unsupported, got %q", resp.Completeness)
	}
	if resp.Session != nil {
		t.Fatal("expected no fabricated session data for unsupported harness")
	}
	if !strings.Contains(resp.Notice, "unsupported") {
		t.Errorf("expected notice mentioning unsupported, got %q", resp.Notice)
	}
}

func TestUnknownView(t *testing.T) {
	_, err := Run("", Options{
		Target: "deepseek:session-abc",
		View:   "nonexistent_view",
	})
	if err == nil {
		t.Fatal("expected error for unknown view")
	}
}

func TestBareTargetResolution(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "myproj", "session-target-test-123")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(sessionDir, "session.v3.jsonl")
	header := `{"type":"session","version":3,"id":"session-target-test-123","createdAt":1700000000000,"cwd":"/workspace/myproj"}
`
	if err := os.WriteFile(logPath, []byte(header), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEJA_DEEPSEEK_ROOT", tmpDir)

	resp, err := Run("", Options{
		Target: "target-test-123",
		View:   "session",
	})
	if err != nil {
		t.Fatalf("Run failed for bare target: %v", err)
	}
	if resp.Target.Harness != "deepseek" {
		t.Errorf("resp.Target.Harness = %q, want deepseek", resp.Target.Harness)
	}
	if resp.Session == nil || resp.Session.ID != "session-target-test-123" {
		t.Errorf("resp.Session = %#v, want ID session-target-test-123", resp.Session)
	}
}

func TestPolicyEnforcement(t *testing.T) {
	tmpDir := t.TempDir()
	policyFile := filepath.Join(tmpDir, "policy.json")

	// Deny project "secret-project"
	polJSON := `{
		"activations": {
			"mcp": {
				"local": true
			}
		},
		"ignore": ["*/secret-project/*"]
	}`
	if err := os.WriteFile(policyFile, []byte(polJSON), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEJA_POLICY_FILE", policyFile)
	t.Setenv("DEJA_DEEPSEEK_ROOT", tmpDir)

	// Verify policy loaded
	pol := policy.Load()
	if !pol.Ignored("/path/to/secret-project/session.jsonl", "secret-project") {
		t.Fatal("policy did not ignore secret-project")
	}

	// Create a mock log in an ignored directory
	sessionDir := filepath.Join(tmpDir, "secret-project", "session-123")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(sessionDir, "session.v3.jsonl")
	header := `{"type":"session","version":3,"id":"123","createdAt":1727612345000,"cwd":"/tmp/secret-project"}` + "\n"
	if err := os.WriteFile(logPath, []byte(header), 0644); err != nil {
		t.Fatal(err)
	}

	// Wire test hook to find log
	oldFinder := findDeepSeekSessionLog
	findDeepSeekSessionLog = func(id string) (string, error) {
		if id == "123" {
			return logPath, nil
		}
		return "", os.ErrNotExist
	}
	defer func() { findDeepSeekSessionLog = oldFinder }()

	resp, err := Run("", Options{
		Target:     "deepseek:123",
		View:       "session",
		Activation: "mcp",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if resp.Completeness != CompletenessUnknown {
		t.Errorf("expected CompletenessUnknown for withheld session, got %q", resp.Completeness)
	}
	if !strings.Contains(resp.Notice, "withheld") {
		t.Errorf("expected notice to mention withheld, got %q", resp.Notice)
	}
	if resp.Session != nil {
		t.Fatal("session data must not be returned when withheld by policy")
	}
}

func createHermeticTraceFixture(t *testing.T) string {
	tmpDir := t.TempDir()
	t.Setenv("DEJA_DEEPSEEK_ROOT", tmpDir)
	logPath := filepath.Join(tmpDir, "session.v3.jsonl")

	lines := []string{
		// 1. Header
		`{"type":"session","version":3,"id":"test-session-001","createdAt":1727612345000,"cwd":"/workspace/test-repo","parentSession":"parent-999","isSeeded":true,"delegationDepth":1,"agentPreset":"standard"}`,
		// 2. End seed event marking inherited cut
		`{"type":"session/end-seed","seq":0,"time":1727612345050,"data":{"inherited":true}}`,
		// 3. Turn 0 start
		`{"type":"turn/start","seq":1,"time":1727612345100,"data":{"turn":0}}`,
		// 4. User message with secret token to test redaction
		`{"type":"user/message","seq":2,"time":1727612345200,"surfaceOp":"append","data":{"role":"user","content":"Run tests with token sk-ant-api03-abcdefghijklmnopqrstuvwxyz1234567890 please"}}`,
		// 5. Step 0 start
		`{"type":"step/start","seq":3,"time":1727612345250,"data":{"turn":0,"step":0}}`,
		// 6. Tool call (bash)
		`{"type":"tool/call","seq":4,"time":1727612345300,"data":{"turn":0,"step":0,"callId":"call-101","name":"bash","arguments":{"command":"ls -la /workspace/test-repo/src/main.go"}}}`,
		// 7. Tool result (bash ok)
		`{"type":"tool/result","seq":5,"time":1727612345400,"surfaceOp":"append","sourceEventSeqs":[4],"data":{"turn":0,"step":0,"callId":"call-101","content":"-rw-r--r-- 1 root root 1200 main.go"}}`,
		// 8. Tool call 2 (web_fetch)
		`{"type":"tool/call","seq":6,"time":1727612345450,"data":{"turn":0,"step":0,"callId":"call-102","name":"web_fetch","arguments":{"url":"https://example.com/api/v1/health"}}}`,
		// 9. Tool result 2 (web_fetch error)
		`{"type":"tool/result","seq":7,"time":1727612345500,"surfaceOp":"append","sourceEventSeqs":[6],"data":{"turn":0,"step":0,"callId":"call-102","error":"connection refused [exit code: 2]"}}`,
		// 10. Tool call 3 (read file)
		`{"type":"tool/call","seq":8,"time":1727612345520,"data":{"turn":0,"step":0,"callId":"call-103","name":"read","arguments":{"file_path":"/workspace/test-repo/src/main.go"}}}`,
		// 11. Tool result 3 (read file ok)
		`{"type":"tool/result","seq":9,"time":1727612345550,"surfaceOp":"append","sourceEventSeqs":[8],"data":{"turn":0,"step":0,"callId":"call-103","content":"package main\nfunc main() {}"}}`,
		// 12. Compaction start
		`{"type":"compaction/start","seq":10,"time":1727612345600,"data":{"compactionId":"cmp-01","turn":0}}`,
		// 13. Compaction summary with surface replacement
		`{"type":"compaction/summary","seq":11,"time":1727612345700,"surfaceOp":{"op":"replace","startSeq":2,"endSeq":9},"data":{"compactionId":"cmp-01","summary":"User requested test run; listed files and queried health endpoint.","shadowedRange":{"start":2,"end":9},"shadowedSeqs":[2,3,4,5,6,7,8,9],"shadowedTokenCount":1500}}`,
		// 14. Compaction end
		`{"type":"compaction/end","seq":12,"time":1727612345800,"data":{"compactionId":"cmp-01","turn":0}}`,
		// 15. Step 0 end
		`{"type":"step/end","seq":13,"time":1727612345850,"data":{"turn":0,"step":0}}`,
		// 16. Turn 0 end
		`{"type":"turn/end","seq":14,"time":1727612345900,"data":{"turn":0,"reason":"completed"}}`,
		// 17. Turn 1 start (open turn test)
		`{"type":"turn/start","seq":15,"time":1727612346000,"data":{"turn":1}}`,
	}

	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return logPath
}

func TestSessionViewProjection(t *testing.T) {
	logPath := createHermeticTraceFixture(t)

	oldFinder := findDeepSeekSessionLog
	findDeepSeekSessionLog = func(id string) (string, error) {
		return logPath, nil
	}
	defer func() { findDeepSeekSessionLog = oldFinder }()

	resp, err := Run("", Options{
		Target: "deepseek:test-session-001",
		View:   "session",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if resp.Completeness != CompletenessComplete {
		t.Errorf("expected CompletenessComplete, got %q", resp.Completeness)
	}
	s := resp.Session
	if s == nil {
		t.Fatal("expected non-nil SessionView")
	}

	if s.ID != "test-session-001" {
		t.Errorf("ID = %q, want test-session-001", s.ID)
	}
	if s.Version != 3 {
		t.Errorf("Version = %d, want 3", s.Version)
	}
	if s.ParentSession != "parent-999" {
		t.Errorf("ParentSession = %q, want parent-999", s.ParentSession)
	}
	if !s.IsSeeded {
		t.Error("expected IsSeeded = true")
	}
	if s.DelegationDepth != 1 {
		t.Errorf("DelegationDepth = %d, want 1", s.DelegationDepth)
	}
	if s.Project != "test-repo" {
		t.Errorf("Project = %q, want test-repo", s.Project)
	}
	if s.TotalEvents != 16 {
		t.Errorf("TotalEvents = %d, want 16", s.TotalEvents)
	}
	if s.TurnCount != 2 {
		t.Errorf("TurnCount = %d, want 2", s.TurnCount)
	}
	if s.ToolCallCount != 3 {
		t.Errorf("ToolCallCount = %d, want 3", s.ToolCallCount)
	}
	if s.Compactions != 1 {
		t.Errorf("Compactions = %d, want 1", s.Compactions)
	}
	if !s.OpenTurn {
		t.Error("expected OpenTurn = true because turn 1 has no turn/end")
	}
}

func TestTimelineViewProjectionAndPagination(t *testing.T) {
	logPath := createHermeticTraceFixture(t)

	oldFinder := findDeepSeekSessionLog
	findDeepSeekSessionLog = func(id string) (string, error) {
		return logPath, nil
	}
	defer func() { findDeepSeekSessionLog = oldFinder }()

	// 1. First page: limit 4
	resp1, err := Run("", Options{
		Target: "deepseek:test-session-001",
		View:   "timeline",
		Limit:  4,
	})
	if err != nil {
		t.Fatalf("Run page 1 failed: %v", err)
	}
	if resp1.Completeness != CompletenessTruncated {
		t.Errorf("expected CompletenessTruncated on first page, got %q", resp1.Completeness)
	}
	if len(resp1.Timeline.Items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(resp1.Timeline.Items))
	}
	if resp1.NextCursor == "" {
		t.Fatal("expected non-empty NextCursor")
	}

	// Verify redaction in item 2 (user/message)
	userMsgItem := resp1.Timeline.Items[2]
	if userMsgItem.Type != "user/message" {
		t.Errorf("item 2 type = %q, want user/message", userMsgItem.Type)
	}
	if strings.Contains(userMsgItem.Summary, "sk-ant-api03") {
		t.Errorf("secret token was not redacted from summary: %q", userMsgItem.Summary)
	}

	// Verify stable ref format
	if !strings.Contains(userMsgItem.Ref, "session.v3.jsonl#seq=2#") {
		t.Errorf("Ref format mismatch: %q", userMsgItem.Ref)
	}

	// 2. Second page using cursor
	resp2, err := Run("", Options{
		Target: "deepseek:test-session-001",
		View:   "timeline",
		Cursor: resp1.NextCursor,
		Limit:  100,
	})
	if err != nil {
		t.Fatalf("Run page 2 failed: %v", err)
	}
	if resp2.Completeness != CompletenessComplete {
		t.Errorf("expected CompletenessComplete on last page, got %q", resp2.Completeness)
	}
	if resp2.NextCursor != "" {
		t.Errorf("expected empty NextCursor on last page, got %q", resp2.NextCursor)
	}

	// Check compaction replacement item in page 2
	var foundCompaction bool
	for _, it := range resp2.Timeline.Items {
		if it.Type == "compaction/summary" {
			foundCompaction = true
			if it.SurfaceOp != "replace" {
				t.Errorf("compaction SurfaceOp = %q, want replace", it.SurfaceOp)
			}
			if it.ReplacesRange == nil || (*it.ReplacesRange)[0] != 2 || (*it.ReplacesRange)[1] != 9 {
				t.Errorf("ReplacesRange mismatch: %v", it.ReplacesRange)
			}
			if it.Compaction == nil || it.Compaction.CompactionID != "cmp-01" {
				t.Errorf("Compaction fact missing or bad ID: %v", it.Compaction)
			}
			if it.Compaction.ShadowedRange == nil || (*it.Compaction.ShadowedRange)[0] != 2 || (*it.Compaction.ShadowedRange)[1] != 9 {
				t.Errorf("Compaction ShadowedRange mismatch: %v", it.Compaction.ShadowedRange)
			}
		}
	}
	if !foundCompaction {
		t.Error("compaction/summary item not found in second page")
	}
}

func TestResourcesViewProjection(t *testing.T) {
	logPath := createHermeticTraceFixture(t)

	oldFinder := findDeepSeekSessionLog
	findDeepSeekSessionLog = func(id string) (string, error) {
		return logPath, nil
	}
	defer func() { findDeepSeekSessionLog = oldFinder }()

	resp, err := Run("", Options{
		Target: "deepseek:test-session-001",
		View:   "resources",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	r := resp.Resources
	if r == nil {
		t.Fatal("expected non-nil ResourcesView")
	}

	// Tools
	if len(r.Tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(r.Tools))
	}
	toolNames := map[string]int{}
	for _, t := range r.Tools {
		toolNames[t.Name] = t.Calls
	}
	if toolNames["bash"] != 1 || toolNames["web_fetch"] != 1 || toolNames["read"] != 1 {
		t.Errorf("unexpected tool counts: %v", toolNames)
	}

	// Commands
	if len(r.Commands) != 1 || !strings.Contains(r.Commands[0], "ls -la") {
		t.Errorf("unexpected commands: %v", r.Commands)
	}

	// Files
	if len(r.Files) != 1 || !strings.Contains(r.Files[0], "main.go") {
		t.Errorf("unexpected files: %v", r.Files)
	}

	// URLs
	if len(r.URLs) != 1 || !strings.Contains(r.URLs[0], "https://example.com") {
		t.Errorf("unexpected URLs: %v", r.URLs)
	}

	// Errors
	if len(r.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d (%v)", len(r.Errors), r.Errors)
	}
	if r.Errors[0].Tool != "web_fetch" {
		t.Errorf("expected error from web_fetch, got %q", r.Errors[0].Tool)
	}
	if !strings.Contains(r.Errors[0].Message, "connection refused") {
		t.Errorf("expected connection refused message, got %q", r.Errors[0].Message)
	}
}

func TestEffectsViewProjection(t *testing.T) {
	logPath := createHermeticTraceFixture(t)

	oldFinder := findDeepSeekSessionLog
	findDeepSeekSessionLog = func(id string) (string, error) {
		return logPath, nil
	}
	defer func() { findDeepSeekSessionLog = oldFinder }()

	resp, err := Run("", Options{
		Target: "deepseek:test-session-001",
		View:   "effects",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	ef := resp.Effects
	if ef == nil {
		t.Fatal("expected non-nil EffectsView")
	}

	// 3 calls + 3 results = 6 effects
	if len(ef.Items) != 6 {
		t.Fatalf("expected 6 effect items, got %d", len(ef.Items))
	}

	// Item 0: requested bash
	if ef.Items[0].Stage != "requested" || ef.Items[0].Tool != "bash" {
		t.Errorf("item 0 mismatch: %v", ef.Items[0])
	}
	// Item 1: reported bash (no error, outcome not inferred as ok)
	if ef.Items[1].Stage != "reported" || ef.Items[1].Tool != "bash" || ef.Items[1].Outcome != "" {
		t.Errorf("item 1 mismatch: %v", ef.Items[1])
	}
	// Item 2: requested web_fetch
	if ef.Items[2].Stage != "requested" || ef.Items[2].Tool != "web_fetch" {
		t.Errorf("item 2 mismatch: %v", ef.Items[2])
	}
	// Item 3: reported web_fetch error
	if ef.Items[3].Stage != "reported" || ef.Items[3].Tool != "web_fetch" || !strings.Contains(ef.Items[3].Outcome, "connection refused") {
		t.Errorf("item 3 mismatch: %v", ef.Items[3])
	}

	// Ensure no synthesized materialized / verified stages
	for _, it := range ef.Items {
		if it.Stage != "requested" && it.Stage != "reported" {
			t.Fatalf("illegal synthesized stage %q", it.Stage)
		}
	}
}

func TestLineageViewProjection(t *testing.T) {
	logPath := createHermeticTraceFixture(t)

	oldFinder := findDeepSeekSessionLog
	findDeepSeekSessionLog = func(id string) (string, error) {
		return logPath, nil
	}
	defer func() { findDeepSeekSessionLog = oldFinder }()

	resp, err := Run("", Options{
		Target: "deepseek:test-session-001",
		View:   "lineage",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	lin := resp.Lineage
	if lin == nil {
		t.Fatal("expected non-nil LineageView")
	}

	if lin.ParentSession != "parent-999" {
		t.Errorf("ParentSession = %q, want parent-999", lin.ParentSession)
	}
	if !lin.IsSeeded {
		t.Error("expected IsSeeded = true")
	}
	if lin.DelegationDepth != 1 {
		t.Errorf("DelegationDepth = %d, want 1", lin.DelegationDepth)
	}
	if lin.InheritedPrefix != 0 {
		t.Errorf("InheritedPrefix = %d, want 0", lin.InheritedPrefix)
	}
}

func TestResponseJSONMarshal(t *testing.T) {
	logPath := createHermeticTraceFixture(t)

	oldFinder := findDeepSeekSessionLog
	findDeepSeekSessionLog = func(id string) (string, error) {
		return logPath, nil
	}
	defer func() { findDeepSeekSessionLog = oldFinder }()

	views := []string{"session", "timeline", "lineage", "resources", "effects", "compactions"}
	for _, v := range views {
		resp, err := Run("", Options{
			Target: "deepseek:test-session-001",
			View:   v,
		})
		if err != nil {
			t.Fatalf("Run(%q) failed: %v", v, err)
		}

		b, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("json.Marshal(resp for %q) failed: %v", v, err)
		}
		if len(b) == 0 {
			t.Fatalf("marshaled json for %q is empty", v)
		}

		var roundTrip Response
		if err := json.Unmarshal(b, &roundTrip); err != nil {
			t.Fatalf("json.Unmarshal(resp for %q) failed: %v", v, err)
		}
		if roundTrip.View != v {
			t.Errorf("roundTrip.View = %q, want %q", roundTrip.View, v)
		}
	}
}

func TestCompactionsViewProjection(t *testing.T) {
	logPath := createHermeticTraceFixture(t)

	oldFinder := findDeepSeekSessionLog
	findDeepSeekSessionLog = func(id string) (string, error) {
		return logPath, nil
	}
	defer func() { findDeepSeekSessionLog = oldFinder }()

	resp, err := Run("", Options{
		Target: "deepseek:test-session-001",
		View:   "compactions",
	})
	if err != nil {
		t.Fatalf("Run compactions failed: %v", err)
	}
	if resp.Compactions == nil {
		t.Fatal("expected non-nil CompactionsView")
	}
	if resp.Compactions.TotalCount != 1 {
		t.Fatalf("TotalCount = %d, want 1", resp.Compactions.TotalCount)
	}
	c := resp.Compactions.Compactions[0]
	if c.CompactionID != "cmp-01" {
		t.Errorf("CompactionID = %q, want cmp-01", c.CompactionID)
	}
	if !strings.Contains(c.SummaryText, "User requested test run") {
		t.Errorf("SummaryText = %q, want User requested test run", c.SummaryText)
	}
	if c.ShadowedRange == nil || c.ShadowedRange[0] != 2 || c.ShadowedRange[1] != 9 {
		t.Errorf("ShadowedRange = %v, want [2, 9]", c.ShadowedRange)
	}

	// Test filter matches
	respFiltered, err := Run("", Options{
		Target: "deepseek:test-session-001",
		View:   "compactions",
		Filter: "health",
	})
	if err != nil || respFiltered.Compactions.TotalCount != 1 {
		t.Errorf("filter health: count = %d, want 1", respFiltered.Compactions.TotalCount)
	}

	// Test filter non-match
	respNoMatch, err := Run("", Options{
		Target: "deepseek:test-session-001",
		View:   "compactions",
		Filter: "nonexistent",
	})
	if err != nil || respNoMatch.Compactions.TotalCount != 0 {
		t.Errorf("filter nonexistent: count = %d, want 0", respNoMatch.Compactions.TotalCount)
	}
}

func TestTimelineFiltering(t *testing.T) {
	logPath := createHermeticTraceFixture(t)

	oldFinder := findDeepSeekSessionLog
	findDeepSeekSessionLog = func(id string) (string, error) {
		return logPath, nil
	}
	defer func() { findDeepSeekSessionLog = oldFinder }()

	// Role filter: user only
	respUser, err := Run("", Options{
		Target: "deepseek:test-session-001",
		View:   "timeline",
		Role:   "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range respUser.Timeline.Items {
		if it.Role != "user" && it.Type != "user/message" {
			t.Errorf("unexpected non-user item: %+v", it)
		}
	}

	// Tool filter: bash only
	respBash, err := Run("", Options{
		Target: "deepseek:test-session-001",
		View:   "timeline",
		Tool:   "bash",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range respBash.Timeline.Items {
		if !strings.Contains(it.Tool, "bash") {
			t.Errorf("unexpected non-bash tool item: %+v", it)
		}
	}

	// Text filter: main.go
	respFilter, err := Run("", Options{
		Target: "deepseek:test-session-001",
		View:   "timeline",
		Filter: "main.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(respFilter.Timeline.Items) == 0 {
		t.Error("expected at least 1 item matching 'main.go'")
	}
}

func TestSpineViewProjection(t *testing.T) {
	logPath := createHermeticTraceFixture(t)

	oldFinder := findDeepSeekSessionLog
	findDeepSeekSessionLog = func(id string) (string, error) {
		return logPath, nil
	}
	defer func() { findDeepSeekSessionLog = oldFinder }()

	resp, err := Run("", Options{
		Target: "deepseek:test-session-001",
		View:   "spine",
	})
	if err != nil {
		t.Fatalf("Run spine failed: %v", err)
	}
	if resp.Spine == nil {
		t.Fatal("expected non-nil SpineView")
	}
	if resp.Spine.TotalTurns != 1 {
		t.Fatalf("TotalTurns = %d, want 1", resp.Spine.TotalTurns)
	}
	turn0 := resp.Spine.Turns[0]
	if turn0.Turn != 0 {
		t.Errorf("Turn = %d, want 0", turn0.Turn)
	}
	if !strings.Contains(turn0.UserPrompt, "Run tests with token") {
		t.Errorf("UserPrompt = %q, want Run tests with token", turn0.UserPrompt)
	}
	if turn0.ToolCallCount != 3 {
		t.Errorf("ToolCallCount = %d, want 3", turn0.ToolCallCount)
	}
}

func TestStateViewProjection(t *testing.T) {
	logPath := createHermeticTraceFixture(t)

	oldFinder := findDeepSeekSessionLog
	findDeepSeekSessionLog = func(id string) (string, error) {
		return logPath, nil
	}
	defer func() { findDeepSeekSessionLog = oldFinder }()

	resp, err := Run("", Options{
		Target: "deepseek:test-session-001",
		View:   "state",
	})
	if err != nil {
		t.Fatalf("Run state failed: %v", err)
	}
	if resp.State == nil {
		t.Fatal("expected non-nil StateView")
	}
	if resp.State.TotalCompacted != 1 {
		t.Errorf("TotalCompacted = %d, want 1", resp.State.TotalCompacted)
	}
	if len(resp.State.Compactions) != 1 {
		t.Errorf("len(Compactions) = %d, want 1", len(resp.State.Compactions))
	}
}

func TestHardenedEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("DEJA_DEEPSEEK_ROOT", tmpDir)
	logPath := filepath.Join(tmpDir, "session.v3.jsonl")

	lines := []string{
		`{"type":"session","version":3,"id":"test-hardened-001","createdAt":1727612345000,"cwd":"/workspace/test-repo"}`,
		// Real user message
		`{"type":"user/message","seq":1,"time":1727612345100,"data":{"content":"real user query","source":{"kind":"user"}}}`,
		// Plugin spliced message (must be suppressed from timeline)
		`{"type":"user/message","seq":2,"time":1727612345150,"data":{"content":"<system-reminder>plugin context</system-reminder>","source":{"kind":"plugin","plugin":"test-plugin"}}}`,
		// Reasoning chunks (must be suppressed from timeline)
		`{"type":"reasoning-chunks","seq0":3,"time0":1727612345200,"data":{"texts":["thinking..."]}}`,
		// Tool call
		`{"type":"tool/call","seq":4,"time":1727612345300,"data":{"callId":"c1","name":"bash","arguments":{"command":"echo hello"}}}`,
		// Tool result ok (outcome must not be inferred as "ok")
		`{"type":"tool/result","seq":5,"time":1727612345400,"data":{"callId":"c1","content":"hello\n"}}`,
		// Structured turn/end error
		`{"type":"turn/end","seq":6,"time":1727612345500,"data":{"turn":1,"reason":{"kind":"error","error":"context overflow"}}}`,
	}

	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	oldFinder := findDeepSeekSessionLog
	findDeepSeekSessionLog = func(id string) (string, error) {
		return logPath, nil
	}
	defer func() { findDeepSeekSessionLog = oldFinder }()

	// 1. Verify timeline suppresses plugin message and reasoning chunks
	respTL, err := Run("", Options{
		Target: "deepseek:test-hardened-001",
		View:   "timeline",
	})
	if err != nil {
		t.Fatalf("Run(timeline) failed: %v", err)
	}
	for _, it := range respTL.Timeline.Items {
		if strings.Contains(it.Summary, "system-reminder") {
			t.Errorf("plugin message was not suppressed from timeline: %q", it.Summary)
		}
		if it.Type == "reasoning-chunks" {
			t.Errorf("reasoning-chunks appeared on timeline: %v", it)
		}
		if it.Type == "turn/end" && !strings.Contains(it.Summary, "error") {
			t.Errorf("turn/end summary did not extract structured error reason: %q", it.Summary)
		}
	}

	// 2. Verify effects view does not infer outcome: "ok"
	respEF, err := Run("", Options{
		Target: "deepseek:test-hardened-001",
		View:   "effects",
	})
	if err != nil {
		t.Fatalf("Run(effects) failed: %v", err)
	}
	if len(respEF.Effects.Items) != 2 {
		t.Fatalf("expected 2 effects, got %d", len(respEF.Effects.Items))
	}
	if respEF.Effects.Items[1].Stage != "reported" || respEF.Effects.Items[1].Outcome != "" {
		t.Errorf("expected reported effect with empty (not inferred 'ok') outcome, got %q", respEF.Effects.Items[1].Outcome)
	}

	// 3. Verify resources view captures structured turn/end error
	respRes, err := Run("", Options{
		Target: "deepseek:test-hardened-001",
		View:   "resources",
	})
	if err != nil {
		t.Fatalf("Run(resources) failed: %v", err)
	}
	foundErr := false
	for _, e := range respRes.Resources.Errors {
		if strings.Contains(e.Message, "context overflow") {
			foundErr = true
			break
		}
	}
	if !foundErr {
		t.Errorf("expected structured turn/end error in resources, got %v", respRes.Resources.Errors)
	}
}
