package inspect

import (
	"encoding/json"





	"strings"







)

func projectState(trace *DeepSeekTrace, filter string) *StateView {
	compView := projectCompactions(trace, filter)
	fullSpine := projectSpine(trace, filter)

	lastCompactedSeq := -1
	for _, c := range compView.Compactions {
		if c.ShadowedRange != nil && c.ShadowedRange[1] > lastCompactedSeq {
			lastCompactedSeq = c.ShadowedRange[1]
		}
		if c.Seq > lastCompactedSeq {
			lastCompactedSeq = c.Seq
		}
	}

	var liveTail []SpineTurn
	if lastCompactedSeq < 0 {
		liveTail = fullSpine.Turns
	} else {
		for _, t := range fullSpine.Turns {
			if t.UserSeq > lastCompactedSeq || t.AssistantSeq > lastCompactedSeq {
				liveTail = append(liveTail, t)
			}
		}
	}

	if liveTail == nil {
		liveTail = make([]SpineTurn, 0)
	}

	tailStart := 0
	if lastCompactedSeq >= 0 {
		tailStart = lastCompactedSeq + 1
	}

	return &StateView{
		SessionID:      trace.Header.ID,
		Compactions:    compView.Compactions,
		TotalCompacted: compView.TotalCount,
		LiveTail:       liveTail,
		TailStartsSeq:  tailStart,
		TotalTurns:     fullSpine.TotalTurns,
	}
}

func extractUserPromptText(rawData json.RawMessage) (string, bool) {
	var d struct {
		Role    string          `json:"role"`
		Source  struct {
			Kind string `json:"kind"`
		} `json:"source"`
		Content any             `json:"content"`
		Message json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(rawData, &d); err != nil {
		return "", false
	}
	if d.Source.Kind != "" && d.Source.Kind != "user" {
		return "", false
	}
	if d.Content != nil {
		if str, ok := d.Content.(string); ok {
			return strings.TrimSpace(str), true
		}
		if blocks, ok := d.Content.([]any); ok {
			var parts []string
			for _, b := range blocks {
				if bm, ok := b.(map[string]any); ok {
					bType, _ := bm["type"].(string)
					if bType == "reasoning" || bType == "thought" {
						continue
					}
					if txt, ok := bm["text"].(string); ok && txt != "" {
						parts = append(parts, txt)
					}
				} else if str, ok := b.(string); ok && str != "" {
					parts = append(parts, str)
				}
			}
			if len(parts) > 0 {
				return strings.TrimSpace(strings.Join(parts, "\n\n")), true
			}
		}
	}
	if len(d.Message) > 0 {
		txt := extractMessageText(d.Message, nil)
		if txt != "" {
			return txt, true
		}
	}
	return "", true
}
