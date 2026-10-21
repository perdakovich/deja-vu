package inspect

import (
	"encoding/json"
	"fmt"




	"strings"







)

func projectTimeline(trace *DeepSeekTrace, cursor string, limit int, filter, roleFilter, toolFilter string) (*TimelineView, string, string) {
	limit = clampLimit(limit)
	startSeq, hasCursor := parseCursor(cursor)
	filterLow := strings.ToLower(strings.TrimSpace(filter))
	roleLow := strings.ToLower(strings.TrimSpace(roleFilter))
	toolLow := strings.ToLower(strings.TrimSpace(toolFilter))

	// Correlate tool calls for tool name resolution
	callTools := make(map[string]string)
	callToolsBySeq := make(map[int]string)
	for _, ev := range trace.Events {
		if ev.Type == "tool/call" {
			var cd struct {
				CallID string `json:"callId"`
				Name   string `json:"name"`
			}
			if err := json.Unmarshal(ev.Data, &cd); err == nil {
				if cd.CallID != "" {
					callTools[cd.CallID] = cd.Name
				}
				callToolsBySeq[ev.Seq] = cd.Name
			}
		}
	}

	var displayable []DeepSeekEvent
	for _, ev := range trace.Events {
		if isDisplayableTimelineEvent(ev) {
			displayable = append(displayable, ev)
		}
	}

	startIdx := 0
	if hasCursor {
		startIdx = len(displayable)
		for i, ev := range displayable {
			if ev.Seq >= startSeq {
				startIdx = i
				break
			}
		}
	}

	var items []TimelineItem
	nextCursor := ""
	completeness := CompletenessComplete
	if trace.Truncated {
		completeness = CompletenessTruncated
	}

	for i := startIdx; i < len(displayable); i++ {
		ev := displayable[i]
		item := buildTimelineItem(ev, trace.Log, callTools, callToolsBySeq)

		if roleLow != "" && strings.ToLower(item.Role) != roleLow && strings.ToLower(item.Type) != roleLow {
			continue
		}
		if toolLow != "" && !strings.Contains(strings.ToLower(item.Tool), toolLow) {
			continue
		}
		if filterLow != "" && !strings.Contains(strings.ToLower(item.Summary), filterLow) &&
			!strings.Contains(strings.ToLower(item.Type), filterLow) &&
			!strings.Contains(strings.ToLower(item.Tool), filterLow) {
			continue
		}

		if len(items) >= limit {
			nextCursor = fmt.Sprintf("seq:%d", ev.Seq)
			completeness = CompletenessTruncated
			break
		}
		items = append(items, item)
	}

	return &TimelineView{
		Items:      items,
		TotalCount: len(displayable),
	}, nextCursor, completeness
}

func buildTimelineItem(ev DeepSeekEvent, logName string, callTools map[string]string, callToolsBySeq map[int]string) TimelineItem {
	item := TimelineItem{
		Seq:             ev.Seq,
		Type:            ev.Type,
		Time:            ev.Time,
		Ref:             formatRef(logName, ev.Seq, ev.RawHash),
		SourceEventSeqs: ev.SourceEventSeqs,
	}

	op, startSeq, endSeq := parseSurfaceOp(ev.SurfaceOp)
	if op == "replace" {
		item.SurfaceOp = "replace"
		item.ReplacesRange = &[2]int{startSeq, endSeq}
	} else if op == "append" {
		item.SurfaceOp = "append"
	}

	var common struct {
		Turn        *int            `json:"turn"`
		Step        *int            `json:"step"`
		Role        string          `json:"role"`
		Reason      string          `json:"reason"`
		CallID      string          `json:"callId"`
		Name        string          `json:"name"`
		Arguments   json.RawMessage `json:"arguments"`
		Error       json.RawMessage `json:"error"`
		Message     json.RawMessage `json:"message"`
		Content     json.RawMessage `json:"content"`
		Compaction  json.RawMessage `json:"compaction"`
		SummaryText string          `json:"summary"`
	}
	_ = json.Unmarshal(ev.Data, &common)

	if common.Turn != nil {
		item.Turn = *common.Turn
	}
	if common.Step != nil {
		item.Step = *common.Step
	}

	switch ev.Type {
	case "turn/start":
		item.Summary = fmt.Sprintf("Turn %d started", item.Turn)
	case "turn/end":
		kind, _ := parseTurnEndReason(ev.Data)
		item.Summary = fmt.Sprintf("Turn %d ended (%s)", item.Turn, kind)
	case "step/start":
		item.Summary = fmt.Sprintf("Step %d started", item.Step)
	case "step/end":
		item.Summary = fmt.Sprintf("Step %d ended", item.Step)
	case "tool/call":
		item.Tool = common.Name
		item.CallID = common.CallID
		argsText := formatArgsPreview(common.Arguments)
		if argsText != "" {
			item.Summary = sanitizeString(fmt.Sprintf("%s(%s)", item.Tool, argsText), 300)
		} else {
			item.Summary = sanitizeString(item.Tool, 300)
		}
	case "tool/result":
		item.CallID = common.CallID
		if item.CallID == "" && len(common.Message) > 0 {
			var msg struct {
				ToolCallID string `json:"toolCallId"`
				Source     struct {
					CallID string `json:"callId"`
				} `json:"source"`
				ID string `json:"id"`
			}
			_ = json.Unmarshal(common.Message, &msg)
			if msg.ToolCallID != "" {
				item.CallID = msg.ToolCallID
			} else if msg.Source.CallID != "" {
				item.CallID = msg.Source.CallID
			} else {
				item.CallID = msg.ID
			}
		}
		if item.Tool == "" && item.CallID != "" {
			item.Tool = callTools[item.CallID]
		}
		if item.Tool == "" && len(ev.SourceEventSeqs) > 0 {
			item.Tool = callToolsBySeq[ev.SourceEventSeqs[0]]
		}
		resText := extractResultText(common.Error, common.Message, common.Content)
		item.Summary = sanitizeString(resText, 300)
	case "assistant/message":
		item.Role = "assistant"
		msgText := extractMessageText(common.Message, common.Content)
		item.Summary = sanitizeString(msgText, 300)
	case "user/message":
		item.Role = "user"
		msgText := extractMessageText(common.Message, common.Content)
		item.Summary = sanitizeString(msgText, 300)
	case "system/message":
		item.Role = "system"
		msgText := extractMessageText(common.Message, common.Content)
		item.Summary = sanitizeString(msgText, 300)
	case "compaction/start":
		var cd struct {
			CompactionID string `json:"compactionId"`
		}
		_ = json.Unmarshal(ev.Data, &cd)
		item.Compaction = &CompactionFact{
			CompactionID: cd.CompactionID,
			Kind:         "start",
		}
		item.Summary = fmt.Sprintf("Compaction %s started", cd.CompactionID)
	case "compaction/prune":
		var cd struct {
			ShadowedSeqs       []int           `json:"shadowedSeqs"`
			ShadowedRange      json.RawMessage `json:"shadowedRange"`
			ShadowedTokenCount int             `json:"shadowedTokenCount"`
		}
		_ = json.Unmarshal(ev.Data, &cd)
		item.Compaction = &CompactionFact{
			Kind:               "prune",
			ShadowedRange:      parseShadowedRange(cd.ShadowedRange),
			ShadowedTokenCount: cd.ShadowedTokenCount,
			ShadowedSeqs:       cd.ShadowedSeqs,
		}
		item.Summary = fmt.Sprintf("Pruned %d shadowed tokens", cd.ShadowedTokenCount)
	case "compaction/summary":
		var cd struct {
			CompactionID       string          `json:"compactionId"`
			Summary            string          `json:"summary"`
			ShadowedRange      json.RawMessage `json:"shadowedRange"`
			ShadowedSeqs       []int           `json:"shadowedSeqs"`
			ShadowedTokenCount int             `json:"shadowedTokenCount"`
		}
		_ = json.Unmarshal(ev.Data, &cd)
		item.Compaction = &CompactionFact{
			CompactionID:       cd.CompactionID,
			Kind:               "summary",
			ShadowedRange:      parseShadowedRange(cd.ShadowedRange),
			ShadowedSeqs:       cd.ShadowedSeqs,
			ShadowedTokenCount: cd.ShadowedTokenCount,
		}
		item.Summary = sanitizeString("Compaction summary: "+cd.Summary, 300)
	case "compaction/end":
		var cd struct {
			CompactionID string `json:"compactionId"`
			Error        string `json:"error"`
		}
		_ = json.Unmarshal(ev.Data, &cd)
		item.Compaction = &CompactionFact{
			CompactionID: cd.CompactionID,
			Kind:         "end",
			Error:        cd.Error,
		}
		if cd.Error != "" {
			item.Summary = fmt.Sprintf("Compaction %s ended with error: %s", cd.CompactionID, sanitizeString(cd.Error, 100))
		} else {
			item.Summary = fmt.Sprintf("Compaction %s completed", cd.CompactionID)
		}
	default:
		item.Summary = sanitizeString(ev.Type, 100)
	}

	return item
}

