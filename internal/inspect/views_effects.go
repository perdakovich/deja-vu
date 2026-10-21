package inspect

import (
	"encoding/json"
	"fmt"


	"sort"
	"strconv"
	"strings"







)

func projectEffects(trace *DeepSeekTrace, cursor string, limit int) (*EffectsView, string, string) {
	limit = clampLimit(limit)
	startSeq, hasCursor := parseCursor(cursor)

	type callInfo struct {
		Name   string
		Target string
	}
	callsByID := make(map[string]callInfo)
	callsBySeq := make(map[int]callInfo)

	var allItems []EffectItem

	for _, ev := range trace.Events {
		switch ev.Type {
		case "tool/call":
			var cd struct {
				CallID    string          `json:"callId"`
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			if err := json.Unmarshal(ev.Data, &cd); err == nil {
				args := parseArgsMap(cd.Arguments)
				target := extractTargetFromArgs(args)
				ci := callInfo{Name: cd.Name, Target: target}
				if cd.CallID != "" {
					callsByID[cd.CallID] = ci
				}
				callsBySeq[ev.Seq] = ci

				item := EffectItem{
					CallID:  sanitizeString(cd.CallID, 100),
					Tool:    sanitizeString(cd.Name, 100),
					Stage:   "requested",
					Seq:     ev.Seq,
					Time:    ev.Time,
					Ref:     formatRef(trace.Log, ev.Seq, ev.RawHash),
					Target:  sanitizeString(target, 200),
					Details: sanitizeString(formatArgsPreview(cd.Arguments), 300),
				}
				allItems = append(allItems, item)
			}
		case "tool/result":
			var rd struct {
				CallID  string          `json:"callId"`
				Error   json.RawMessage `json:"error"`
				Message json.RawMessage `json:"message"`
				Content json.RawMessage `json:"content"`
			}
			if err := json.Unmarshal(ev.Data, &rd); err == nil {
				callID := rd.CallID
				if callID == "" && len(rd.Message) > 0 {
					var msg struct {
						ID string `json:"id"`
					}
					_ = json.Unmarshal(rd.Message, &msg)
					callID = msg.ID
				}

				ci, ok := callsByID[callID]
				if !ok && len(ev.SourceEventSeqs) > 0 {
					ci = callsBySeq[ev.SourceEventSeqs[0]]
				}

				resText := extractResultText(rd.Error, rd.Message, rd.Content)
				isErr, errMsg := checkResultError(rd.Error, resText)
				outcome := ""
				if isErr {
					outcome = errMsg
				}

				item := EffectItem{
					CallID:  sanitizeString(callID, 100),
					Tool:    sanitizeString(ci.Name, 100),
					Stage:   "reported",
					Seq:     ev.Seq,
					Time:    ev.Time,
					Ref:     formatRef(trace.Log, ev.Seq, ev.RawHash),
					Target:  sanitizeString(ci.Target, 200),
					Outcome: sanitizeString(outcome, 100),
					Details: sanitizeString(resText, 300),
				}
				allItems = append(allItems, item)
			}
		}
	}

	startIdx := 0
	if hasCursor {
		startIdx = len(allItems)
		for i, it := range allItems {
			if it.Seq >= startSeq {
				startIdx = i
				break
			}
		}
	}

	var items []EffectItem
	nextCursor := ""
	completeness := CompletenessComplete
	if trace.Truncated {
		completeness = CompletenessTruncated
	}

	for i := startIdx; i < len(allItems); i++ {
		if len(items) >= limit {
			nextCursor = fmt.Sprintf("seq:%d", allItems[i].Seq)
			completeness = CompletenessTruncated
			break
		}
		items = append(items, allItems[i])
	}

	if items == nil {
		items = make([]EffectItem, 0)
	}

	return &EffectsView{
		Items:      items,
		TotalCount: len(allItems),
	}, nextCursor, completeness
}

func parseArgsMap(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err == nil {
		return m
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		var m2 map[string]any
		if err := json.Unmarshal([]byte(str), &m2); err == nil {
			return m2
		}
	}
	return nil
}

func formatArgsPreview(raw json.RawMessage) string {
	m := parseArgsMap(raw)
	if m == nil {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
		return string(raw)
	}
	var pairs []string
	for k, v := range m {
		pairs = append(pairs, fmt.Sprintf("%s=%v", k, v))
	}
	sort.Strings(pairs)
	return strings.Join(pairs, ", ")
}

func extractTargetFromArgs(args map[string]any) string {
	if args == nil {
		return ""
	}
	for _, k := range []string{"command", "cmd", "file_path", "path", "file", "url", "target", "query"} {
		if v, ok := args[k].(string); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func extractFilesFromArgs(args map[string]any, set map[string]bool, files *[]string) {
	if args == nil {
		return
	}
	for _, k := range []string{"file_path", "path", "target", "old_path", "new_path", "file"} {
		if v, ok := args[k].(string); ok && strings.TrimSpace(v) != "" {
			clean := sanitizeString(v, 300)
			if clean != "" && !set[clean] {
				set[clean] = true
				*files = append(*files, clean)
			}
		}
	}
	if fl, ok := args["files"].([]any); ok {
		for _, item := range fl {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				clean := sanitizeString(s, 300)
				if clean != "" && !set[clean] {
					set[clean] = true
					*files = append(*files, clean)
				}
			} else if m, ok := item.(map[string]any); ok {
				if p, ok := m["path"].(string); ok && strings.TrimSpace(p) != "" {
					clean := sanitizeString(p, 300)
					if clean != "" && !set[clean] {
						set[clean] = true
						*files = append(*files, clean)
					}
				}
			}
		}
	}
}

func extractURLsFromArgs(args map[string]any, set map[string]bool, urls *[]string) {
	if args == nil {
		return
	}
	for _, k := range []string{"url", "urls"} {
		if v, ok := args[k].(string); ok && strings.TrimSpace(v) != "" {
			clean := sanitizeString(v, 300)
			if clean != "" && !set[clean] {
				set[clean] = true
				*urls = append(*urls, clean)
			}
		}
	}
	for _, v := range args {
		if s, ok := v.(string); ok {
			for _, match := range urlRE.FindAllString(s, -1) {
				clean := sanitizeString(match, 200)
				if clean != "" && !set[clean] {
					set[clean] = true
					*urls = append(*urls, clean)
				}
			}
		}
	}
}

func extractResultText(rawErr, rawMsg, rawContent json.RawMessage) string {
	if len(rawErr) > 0 {
		var s string
		if err := json.Unmarshal(rawErr, &s); err == nil && s != "" {
			return s
		}
		return string(rawErr)
	}
	if len(rawMsg) > 0 {
		txt := extractMessageText(rawMsg, nil)
		if txt != "" {
			return txt
		}
	}
	if len(rawContent) > 0 {
		var s string
		if err := json.Unmarshal(rawContent, &s); err == nil {
			return s
		}
		var blocks []map[string]any
		if err := json.Unmarshal(rawContent, &blocks); err == nil {
			var parts []string
			for _, b := range blocks {
				if t, ok := b["text"].(string); ok && t != "" {
					parts = append(parts, t)
				}
			}
			return strings.Join(parts, "\n")
		}
	}
	return ""
}

func extractMessageText(rawMsg, rawContent json.RawMessage) string {
	if len(rawMsg) > 0 {
		var m struct {
			Content json.RawMessage `json:"content"`
			Text    string          `json:"text"`
		}
		if err := json.Unmarshal(rawMsg, &m); err == nil {
			if m.Text != "" {
				return m.Text
			}
			if len(m.Content) > 0 {
				return extractMessageText(nil, m.Content)
			}
		}
	}
	if len(rawContent) > 0 {
		var s string
		if err := json.Unmarshal(rawContent, &s); err == nil {
			return s
		}
		var blocks []map[string]any
		if err := json.Unmarshal(rawContent, &blocks); err == nil {
			var parts []string
			for _, b := range blocks {
				bType, _ := b["type"].(string)
				// Skip internal model thinking
				if bType == "reasoning" {
					continue
				}
				if t, ok := b["text"].(string); ok && t != "" {
					parts = append(parts, t)
				}
			}
			return strings.Join(parts, "\n")
		}
	}
	return ""
}

func checkResultError(rawErr json.RawMessage, resText string) (bool, string) {
	if len(rawErr) > 0 {
		var s string
		if err := json.Unmarshal(rawErr, &s); err == nil && s != "" {
			return true, s
		}
		return true, string(rawErr)
	}
	if m := exitCodeRE.FindStringSubmatch(resText); len(m) > 1 {
		code, err := strconv.Atoi(m[1])
		if err == nil && code != 0 {
			return true, fmt.Sprintf("exit code %d", code)
		}
	}
	if strings.Contains(resText, "[sandbox: file access denied") {
		return true, "sandbox denial"
	}
	return false, ""
}

