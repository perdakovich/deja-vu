package inspect

import (
	"encoding/json"
	"fmt"


	"sort"

	"strings"







)

func projectResources(trace *DeepSeekTrace, fields string) *ResourcesView {
	toolCounts := make(map[string]int)
	cmdSet := make(map[string]bool)
	var commands []string
	fileSet := make(map[string]bool)
	var files []string
	urlSet := make(map[string]bool)
	var urls []string
	var errors []ResourceErr

	callTools := make(map[string]string)
	callToolsBySeq := make(map[int]string)

	for _, ev := range trace.Events {
		switch ev.Type {
		case "tool/call":
			var cd struct {
				CallID    string          `json:"callId"`
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			if err := json.Unmarshal(ev.Data, &cd); err == nil {
				if cd.Name != "" {
					toolCounts[cd.Name]++
					if cd.CallID != "" {
						callTools[cd.CallID] = cd.Name
					}
					callToolsBySeq[ev.Seq] = cd.Name
				}
				args := parseArgsMap(cd.Arguments)
				// Commands
				for _, k := range []string{"command", "cmd", "script"} {
					if cmdVal, ok := args[k].(string); ok && strings.TrimSpace(cmdVal) != "" {
						clean := sanitizeString(cmdVal, 300)
						if clean != "" && !cmdSet[clean] {
							cmdSet[clean] = true
							commands = append(commands, clean)
						}
					}
				}
				// Files
				extractFilesFromArgs(args, fileSet, &files)
				// URLs
				extractURLsFromArgs(args, urlSet, &urls)
			}
		case "tool/result":
			var rd struct {
				CallID  string          `json:"callId"`
				Error   json.RawMessage `json:"error"`
				Message json.RawMessage `json:"message"`
				Content json.RawMessage `json:"content"`
			}
			if err := json.Unmarshal(ev.Data, &rd); err == nil {
				toolName := ""
				if rd.CallID != "" {
					toolName = callTools[rd.CallID]
				}
				if toolName == "" && len(ev.SourceEventSeqs) > 0 {
					toolName = callToolsBySeq[ev.SourceEventSeqs[0]]
				}

				resText := extractResultText(rd.Error, rd.Message, rd.Content)
				isErr, errMsg := checkResultError(rd.Error, resText)
				if isErr {
					errors = append(errors, ResourceErr{
						Seq:     ev.Seq,
						Tool:    toolName,
						Ref:     formatRef(trace.Log, ev.Seq, ev.RawHash),
						Message: sanitizeString(errMsg, 300),
					})
				}
				// Scan result text for URLs
				for _, match := range urlRE.FindAllString(resText, -1) {
					clean := sanitizeString(match, 200)
					if clean != "" && !urlSet[clean] {
						urlSet[clean] = true
						urls = append(urls, clean)
					}
				}
			}
		case "turn/end":
			var td struct {
				Turn int `json:"turn"`
			}
			_ = json.Unmarshal(ev.Data, &td)
			kind, detail := parseTurnEndReason(ev.Data)
			if kind == "error" {
				msg := fmt.Sprintf("turn %d ended with error", td.Turn)
				if detail != "" {
					msg = fmt.Sprintf("turn %d ended with error: %s", td.Turn, detail)
				}
				errors = append(errors, ResourceErr{
					Seq:     ev.Seq,
					Ref:     formatRef(trace.Log, ev.Seq, ev.RawHash),
					Message: sanitizeString(msg, 300),
				})
			}
		}
	}

	var tools []ToolUsage
	for name, count := range toolCounts {
		tools = append(tools, ToolUsage{Name: sanitizeString(name, 100), Calls: count})
	}
	sort.Slice(tools, func(i, j int) bool {
		if tools[i].Calls != tools[j].Calls {
			return tools[i].Calls > tools[j].Calls
		}
		return tools[i].Name < tools[j].Name
	})

	if commands == nil {
		commands = make([]string, 0)
	} else if len(commands) > maxResourceCommands {
		commands = commands[:maxResourceCommands]
	}
	if files == nil {
		files = make([]string, 0)
	} else if len(files) > maxResourceFiles {
		files = files[:maxResourceFiles]
	}
	if urls == nil {
		urls = make([]string, 0)
	} else if len(urls) > maxResourceURLs {
		urls = urls[:maxResourceURLs]
	}
	if errors == nil {
		errors = make([]ResourceErr, 0)
	} else if len(errors) > maxResourceErrors {
		errors = errors[:maxResourceErrors]
	}

	fieldsLow := strings.ToLower(strings.TrimSpace(fields))
	if fieldsLow != "" {
		wantsTools := strings.Contains(fieldsLow, "tool")
		wantsCmds := strings.Contains(fieldsLow, "cmd") || strings.Contains(fieldsLow, "command")
		wantsFiles := strings.Contains(fieldsLow, "file")
		wantsURLs := strings.Contains(fieldsLow, "url")
		wantsErrs := strings.Contains(fieldsLow, "err")

		if !wantsTools {
			tools = nil
		}
		if !wantsCmds {
			commands = nil
		}
		if !wantsFiles {
			files = nil
		}
		if !wantsURLs {
			urls = nil
		}
		if !wantsErrs {
			errors = nil
		}
	}

	return &ResourcesView{
		Tools:    tools,
		Commands: commands,
		Files:    files,
		URLs:     urls,
		Errors:   errors,
	}
}

