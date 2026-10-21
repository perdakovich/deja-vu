package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/vshulcz/deja-vu/internal/inspect"
	"github.com/vshulcz/deja-vu/internal/policy"
	"github.com/vshulcz/deja-vu/internal/usage"
)

func runInspect(dir string, args []string, stdout io.Writer) error {
	var target string
	view := "session"
	cursor := ""
	filter := ""
	role := ""
	tool := ""
	fields := ""
	limit := 0
	asJSON := false

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--json", a == "-json":
			asJSON = true
		case a == "--view", a == "-view", a == "-v":
			if i+1 >= len(args) {
				return fmt.Errorf("--view requires an argument (session, timeline, lineage, resources, effects, compactions)")
			}
			i++
			view = args[i]
		case strings.HasPrefix(a, "--view="):
			view = strings.TrimPrefix(a, "--view=")
		case a == "--filter", a == "-filter", a == "-f":
			if i+1 >= len(args) {
				return fmt.Errorf("--filter requires an argument")
			}
			i++
			filter = args[i]
		case strings.HasPrefix(a, "--filter="):
			filter = strings.TrimPrefix(a, "--filter=")
		case a == "--role", a == "-role", a == "-r":
			if i+1 >= len(args) {
				return fmt.Errorf("--role requires an argument (user, assistant, tool, system)")
			}
			i++
			role = args[i]
		case strings.HasPrefix(a, "--role="):
			role = strings.TrimPrefix(a, "--role=")
		case a == "--tool", a == "-tool":
			if i+1 >= len(args) {
				return fmt.Errorf("--tool requires an argument")
			}
			i++
			tool = args[i]
		case strings.HasPrefix(a, "--tool="):
			tool = strings.TrimPrefix(a, "--tool=")
		case a == "--fields", a == "-fields":
			if i+1 >= len(args) {
				return fmt.Errorf("--fields requires an argument")
			}
			i++
			fields = args[i]
		case strings.HasPrefix(a, "--fields="):
			fields = strings.TrimPrefix(a, "--fields=")
		case a == "--cursor", a == "-cursor", a == "-c", a == "--offset", a == "-offset":
			if i+1 >= len(args) {
				return fmt.Errorf("--cursor requires an argument")
			}
			i++
			cursor = args[i]
		case strings.HasPrefix(a, "--cursor="):
			cursor = strings.TrimPrefix(a, "--cursor=")
		case strings.HasPrefix(a, "--offset="):
			cursor = strings.TrimPrefix(a, "--offset=")
		case a == "--limit", a == "-limit", a == "-n":
			if i+1 >= len(args) {
				return fmt.Errorf("--limit requires an integer argument")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				return fmt.Errorf("invalid --limit %q", args[i])
			}
			limit = n
		case strings.HasPrefix(a, "--limit="):
			n, err := strconv.Atoi(strings.TrimPrefix(a, "--limit="))
			if err != nil || n < 1 {
				return fmt.Errorf("invalid --limit %q", a)
			}
			limit = n
		case a == "--help", a == "-help", a == "-h":
			fmt.Fprintln(stdout, `Usage: deja inspect <target> [--view session|state|spine|compactions|timeline|lineage|resources|effects] [--cursor seq] [--filter text] [--role r] [--tool t] [--limit n] [--json]

Inspect session logs, event streams, milestone compactions, dialogue spine, resources, tool effects, and delegation lineage.

Views:
  session      Session-level metadata, turns, and tool call counts (default)
  state        Two-tier state: historical milestone compactions + live uncompacted tail (optimal for agents)
  spine        Pure conversation spine (user prompts + final assistant answers, zero tool exhaust)
  compactions  Compaction checkpoints / milestone summaries across session life
  timeline     Paginated chronological stream of user, assistant, and tool events
  lineage      Session parent/root origin and child fork tree
  resources    Aggregated tool usage, executed commands, files touched, and HTTP URLs
  effects      Sequenced tool call requests and outcomes with replayable hashes

Flags:
  --view <name>     Select view (session, state, spine, compactions, timeline, lineage, resources, effects)
  --cursor <seq>    Pagination cursor / offset (e.g. seq:100 or 100)
  --filter <text>   Filter events, compactions, or spine turns by substring (-f)
  --role <role>     Filter timeline events by role: user, assistant, tool, system (-r)
  --tool <name>     Filter timeline events by tool name (bash, read, edit, etc.)
  --fields <list>   Project specific resource fields (files, commands, tools)
  --limit <n>       Maximum items to return (-n)
  --json            Output raw machine-readable JSON`)
			return nil
		case strings.HasPrefix(a, "-"):
			return fmt.Errorf("unknown inspect flag: %s", a)
		default:
			if target == "" {
				target = a
			} else {
				return fmt.Errorf("unexpected extra argument %q; inspect takes a single target", a)
			}
		}
	}

	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("inspect requires a session target (e.g. deepseek:<id>, deja://session/<id>, or bare UUID)")
	}

	resp, err := inspect.Run(dir, inspect.Options{
		Target:     target,
		View:       view,
		Cursor:     cursor,
		Limit:      limit,
		Filter:     filter,
		Role:       role,
		Tool:       tool,
		Fields:     fields,
		Activation: policy.ActivationInspect,
	})
	if err != nil {
		return err
	}

	if asJSON {
		b, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(b))
		usage.RecordServedFromInto(dir, usage.KindInspect, string(b), "cli", 1, 0, []string{resp.Target.ID}, nil, policy.Load().Describe(policy.ActivationInspect))
		return nil
	}

	formatInspectText(stdout, resp)
	return nil
}

func formatInspectText(w io.Writer, resp inspect.Response) {
	fmt.Fprintf(w, "Target:       %s:%s\n", resp.Target.Harness, resp.Target.ID)
	fmt.Fprintf(w, "View:         %s\n", resp.View)
	fmt.Fprintf(w, "Completeness: %s\n", resp.Completeness)
	if resp.Notice != "" {
		fmt.Fprintf(w, "Notice:       %s\n", resp.Notice)
	}
	if resp.SourceCoverage.LogFile != "" {
		fmt.Fprintf(w, "Log File:     %s (gen %d, %d events)\n",
			resp.SourceCoverage.LogFile, resp.SourceCoverage.Generation, resp.SourceCoverage.TotalEvents)
	}
	fmt.Fprintln(w)

	switch resp.View {
	case "session":
		if s := resp.Session; s != nil {
			if !s.CreatedAt.IsZero() {
				fmt.Fprintf(w, "Created:      %s\n", s.CreatedAt.Format("2006-01-02 15:04:05"))
			}
			if s.Project != "" {
				fmt.Fprintf(w, "Project:      %s\n", s.Project)
			}
			if s.CWD != "" {
				fmt.Fprintf(w, "CWD:          %s\n", s.CWD)
			}
			if s.ParentSession != "" {
				fmt.Fprintf(w, "Parent:       %s\n", s.ParentSession)
			}
			if s.AgentPreset != "" {
				fmt.Fprintf(w, "Agent Preset: %s\n", s.AgentPreset)
			}
			fmt.Fprintf(w, "Turns:        %d\n", s.TurnCount)
			fmt.Fprintf(w, "Tool Calls:   %d\n", s.ToolCallCount)
			fmt.Fprintf(w, "Total Events: %d\n", s.TotalEvents)
			fmt.Fprintf(w, "Compactions:  %d\n", s.Compactions)

			if len(s.AvailableViews) > 1 {
				fmt.Fprintln(w, "\nAvailable drill-downs:")
				for _, v := range s.AvailableViews {
					if v == "session" {
						continue
					}
					hint := ""
					switch v {
					case "compactions":
						hint = fmt.Sprintf("(%d milestone summaries: deja inspect %s:%s --view compactions)", s.Compactions, resp.Target.Harness, resp.Target.ID)
					case "resources":
						hint = "(tool usage, commands, files touched)"
					case "timeline":
						hint = "(chronological stream, use --cursor, --filter, --role)"
					case "lineage":
						hint = "(parent/child delegation tree)"
					case "effects":
						hint = "(tool call arguments and results)"
					}
					fmt.Fprintf(w, "  -> deja inspect %s:%s --view %-12s %s\n", resp.Target.Harness, resp.Target.ID, v, hint)
				}
			}
		}
	case "compactions":
		if c := resp.Compactions; c != nil {
			fmt.Fprintf(w, "Compaction Checkpoints (%d):\n\n", c.TotalCount)
			for i, entry := range c.Compactions {
				rangeStr := ""
				if entry.ShadowedRange != nil {
					rangeStr = fmt.Sprintf(" [seq:%d..%d, %d tokens pruned]", entry.ShadowedRange[0], entry.ShadowedRange[1], entry.ShadowedTokenCount)
				}
				timeStr := ""
				if !entry.Time.IsZero() {
					timeStr = " at " + entry.Time.Format("15:04:05")
				}
				modelStr := ""
				if entry.Model != "" {
					modelStr = fmt.Sprintf(" via %s/%s", entry.Provider, entry.Model)
				}
				fmt.Fprintf(w, "=== Checkpoint #%d (seq:%d%s%s%s) ===\n", i+1, entry.Seq, timeStr, modelStr, rangeStr)
				if entry.SummaryText != "" {
					fmt.Fprintf(w, "%s\n\n", entry.SummaryText)
				}
			}
		}
	case "spine":
		if s := resp.Spine; s != nil {
			fmt.Fprintf(w, "Dialogue Spine (%d turns):\n\n", s.TotalTurns)
			for _, t := range s.Turns {
				timeStr := ""
				if !t.Time.IsZero() {
					timeStr = " at " + t.Time.Format("15:04:05")
				}
				toolsStr := ""
				if t.ToolCallCount > 0 {
					toolsStr = fmt.Sprintf(" [%d tool calls skipped]", t.ToolCallCount)
				}
				fmt.Fprintf(w, "=== Turn %d (seq:%d%s%s) ===\n", t.Turn, t.UserSeq, timeStr, toolsStr)
				if t.UserPrompt != "" {
					fmt.Fprintf(w, "[USER]:\n%s\n\n", t.UserPrompt)
				}
				if t.AssistantAnswer != "" {
					fmt.Fprintf(w, "[ASSISTANT (seq:%d)]:\n%s\n\n", t.AssistantSeq, t.AssistantAnswer)
				}
			}
		}
	case "state":
		if st := resp.State; st != nil {
			fmt.Fprintf(w, "Session State (Total Turns: %d | Compacted Milestones: %d | Live Tail Turns: %d)\n\n",
				st.TotalTurns, st.TotalCompacted, len(st.LiveTail))

			if len(st.Compactions) > 0 {
				fmt.Fprintf(w, "--- TIER 1: HISTORICAL MILESTONES (%d) ---\n\n", len(st.Compactions))
				for i, entry := range st.Compactions {
					rangeStr := ""
					if entry.ShadowedRange != nil {
						rangeStr = fmt.Sprintf(" [seq:%d..%d, %d tokens pruned]", entry.ShadowedRange[0], entry.ShadowedRange[1], entry.ShadowedTokenCount)
					}
					timeStr := ""
					if !entry.Time.IsZero() {
						timeStr = " at " + entry.Time.Format("15:04:05")
					}
					fmt.Fprintf(w, "### Milestone #%d (seq:%d%s%s)\n", i+1, entry.Seq, timeStr, rangeStr)
					if entry.SummaryText != "" {
						fmt.Fprintf(w, "%s\n\n", entry.SummaryText)
					}
				}
			}

			if len(st.LiveTail) > 0 {
				fmt.Fprintf(w, "--- TIER 2: LIVE UNCOMPACTED TAIL (Starts seq:%d, %d turns) ---\n\n",
					st.TailStartsSeq, len(st.LiveTail))
				for _, t := range st.LiveTail {
					timeStr := ""
					if !t.Time.IsZero() {
						timeStr = " at " + t.Time.Format("15:04:05")
					}
					toolsStr := ""
					if t.ToolCallCount > 0 {
						toolsStr = fmt.Sprintf(" [%d tool calls skipped]", t.ToolCallCount)
					}
					fmt.Fprintf(w, "=== Live Turn %d (seq:%d%s%s) ===\n", t.Turn, t.UserSeq, timeStr, toolsStr)
					if t.UserPrompt != "" {
						fmt.Fprintf(w, "[USER]:\n%s\n\n", t.UserPrompt)
					}
					if t.AssistantAnswer != "" {
						fmt.Fprintf(w, "[ASSISTANT (seq:%d)]:\n%s\n\n", t.AssistantSeq, t.AssistantAnswer)
					}
				}
			}
		}
	case "lineage":
		if l := resp.Lineage; l != nil {
			if l.ParentSession != "" {
				fmt.Fprintf(w, "Parent Session: %s\n", l.ParentSession)
			}
			fmt.Fprintf(w, "Delegation Depth: %d\n", l.DelegationDepth)
			if l.Origin != "" {
				fmt.Fprintf(w, "Origin:           %s\n", l.Origin)
			}
			if len(l.Children) > 0 {
				fmt.Fprintf(w, "\nChild Sessions (%d):\n", len(l.Children))
				for _, c := range l.Children {
					fmt.Fprintf(w, "  - %s (%s, updated %s)\n", c.ID, c.Harness, c.Updated.Format("2006-01-02 15:04:05"))
				}
			} else {
				fmt.Fprintln(w, "\nChild Sessions:   none")
			}
		}
	case "resources":
		if r := resp.Resources; r != nil {
			if len(r.Tools) > 0 {
				fmt.Fprintln(w, "Tool Usage:")
				for _, t := range r.Tools {
					fmt.Fprintf(w, "  %-16s %d calls\n", t.Name+":", t.Calls)
				}
				fmt.Fprintln(w)
			}
			if len(r.Commands) > 0 {
				fmt.Fprintf(w, "Commands Executed (%d):\n", len(r.Commands))
				for _, c := range r.Commands {
					fmt.Fprintf(w, "  $ %s\n", c)
				}
				fmt.Fprintln(w)
			}
			if len(r.Files) > 0 {
				fmt.Fprintf(w, "Files Touched (%d):\n", len(r.Files))
				for _, f := range r.Files {
					fmt.Fprintf(w, "  %s\n", f)
				}
				fmt.Fprintln(w)
			}
			if len(r.URLs) > 0 {
				fmt.Fprintf(w, "HTTP URLs (%d):\n", len(r.URLs))
				for _, u := range r.URLs {
					fmt.Fprintf(w, "  %s\n", u)
				}
				fmt.Fprintln(w)
			}
			if len(r.Errors) > 0 {
				fmt.Fprintf(w, "Errors (%d):\n", len(r.Errors))
				for _, errItem := range r.Errors {
					fmt.Fprintf(w, "  [seq:%-4d] %s: %s\n", errItem.Seq, errItem.Tool, errItem.Message)
				}
			}
		}
	case "timeline":
		if t := resp.Timeline; t != nil {
			fmt.Fprintf(w, "Timeline Events (%d displayed):\n\n", len(t.Items))
			for _, item := range t.Items {
				timeStr := ""
				if !item.Time.IsZero() {
					timeStr = item.Time.Format("15:04:05") + " "
				}
				typeLabel := item.Type
				if item.Tool != "" {
					typeLabel = item.Type + ":" + item.Tool
				} else if item.Role != "" {
					typeLabel = item.Role
				}
				fmt.Fprintf(w, "[seq:%-4d %s] %-18s %s\n", item.Seq, timeStr, typeLabel, item.Summary)
			}
			if resp.NextCursor != "" {
				fmt.Fprintf(w, "\nNext Cursor: %s\n", resp.NextCursor)
			}
		}
	case "effects":
		if e := resp.Effects; e != nil {
			fmt.Fprintf(w, "Tool Effects (%d stages):\n\n", len(e.Items))
			for _, item := range e.Items {
				timeStr := ""
				if !item.Time.IsZero() {
					timeStr = item.Time.Format("15:04:05") + " "
				}
				fmt.Fprintf(w, "[seq:%-4d %s] %-10s %s (%s)\n", item.Seq, timeStr, item.Stage, item.Tool, item.CallID)
				if item.Target != "" {
					fmt.Fprintf(w, "  target:  %s\n", item.Target)
				}
				if item.Outcome != "" {
					fmt.Fprintf(w, "  outcome: %s\n", item.Outcome)
				}
				if item.Details != "" {
					fmt.Fprintf(w, "  details: %s\n", sanitizeCLI(item.Details, 120))
				}
			}
			if resp.NextCursor != "" {
				fmt.Fprintf(w, "\nNext Cursor: %s\n", resp.NextCursor)
			}
		}
	}
}

func sanitizeCLI(s string, max int) string {
	single := strings.ReplaceAll(strings.ReplaceAll(s, "\r", ""), "\n", " ")
	if len(single) > max {
		return single[:max] + "..."
	}
	return single
}
