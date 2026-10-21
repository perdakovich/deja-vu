package inspect

import (
	"encoding/json"





	"strings"







)

func projectCompactions(trace *DeepSeekTrace, filter string) *CompactionsView {
	filterLow := strings.ToLower(strings.TrimSpace(filter))
	var entries []CompactionEntry

	for _, ev := range trace.Events {
		if ev.Type == "compaction/summary" {
			var cd struct {
				CompactionID       string          `json:"compactionId"`
				Turn               int             `json:"turn"`
				ShadowedRange      json.RawMessage `json:"shadowedRange"`
				ShadowedTokenCount int             `json:"shadowedTokenCount"`
				ShadowedSeqs       []int           `json:"shadowedSeqs"`
				Provider           string          `json:"provider"`
				Model              string          `json:"model"`
				Summary            json.RawMessage `json:"summary"`
				RawOutput          json.RawMessage `json:"rawOutput"`
			}
			if err := json.Unmarshal(ev.Data, &cd); err == nil {
				summaryText := extractSummaryText(cd.Summary, cd.RawOutput)
				if filterLow != "" && !strings.Contains(strings.ToLower(summaryText), filterLow) {
					continue
				}
				entries = append(entries, CompactionEntry{
					CompactionID:       cd.CompactionID,
					Seq:                ev.Seq,
					Time:               ev.Time,
					Turn:               cd.Turn,
					ShadowedRange:      parseShadowedRange(cd.ShadowedRange),
					ShadowedTokenCount: cd.ShadowedTokenCount,
					ShadowedSeqs:       len(cd.ShadowedSeqs),
					Provider:           cd.Provider,
					Model:              cd.Model,
					SummaryText:        summaryText,
				})
			}
		}
	}

	if entries == nil {
		entries = make([]CompactionEntry, 0)
	}

	return &CompactionsView{
		SessionID:   trace.Header.ID,
		TotalCount:  len(entries),
		Compactions: entries,
	}
}

func extractSummaryText(summaryRaw, rawOutputRaw json.RawMessage) string {
	if len(summaryRaw) > 0 {
		var blocks []map[string]any
		if err := json.Unmarshal(summaryRaw, &blocks); err == nil {
			var parts []string
			for _, b := range blocks {
				if t, ok := b["text"].(string); ok && t != "" {
					parts = append(parts, t)
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, "\n\n")
			}
		}
		var str string
		if err := json.Unmarshal(summaryRaw, &str); err == nil && str != "" {
			return str
		}
	}
	if len(rawOutputRaw) > 0 {
		var str string
		if err := json.Unmarshal(rawOutputRaw, &str); err == nil && str != "" {
			return str
		}
	}
	return ""
}

