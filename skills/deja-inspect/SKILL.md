---
name: deja-inspect
description: Deep hierarchical inspection and traversal of AI coding sessions (events, state snapshots, dialogue spine, milestone compactions, resources, tool effects, and delegation lineage) via deja inspect and MCP. Use after discovering a session with deja search/recall, before investigating historical changes, to zoom into past turn execution without dumping raw logs.
metadata:
  openclaw:
    homepage: https://vshulcz.github.io/deja-vu/guide/commands.html
    requires:
      bins:
        - deja
---

# `deja-inspect` — Hierarchical Session Traversal & Forensics

Use `deja inspect` to navigate and audit historical AI coding sessions across harnesses (DeepSeek Harness, OpenCode, Claude, Cursor, etc.) without blowing your context window on raw transcript dumps.

Never use primitive bash scripts (`zstdcat`, manual `jq`, python log scrapers) to read session traces. `deja inspect` provides bounded, deterministic, multi-tiered inspection in milliseconds.

If the `mcp__deja__deja` tool with `mode: "inspect"` is available in your session, use that directly — same substrate, native JSON.

---

## ⚡ The Progressive Disclosure Hierarchy

```
[ Discovery ]           deja search "<topic>"
                              │  (finds candidate session IDs and snippets)
                              ▼
[ Passport & Triage ]   deja inspect <id> --view session
                              │  (turns, events, compactions, CWD, drill-down menu)
          ┌───────────────────┼───────────────────┐
          ▼                   ▼                   ▼
[ State Continuation ]  [ Dialogue Spine ]  [ Chapter Milestones ]
  --view state            --view spine        --view compactions
  (Milestones +           (Human intent +     (All summaries &
   uncompacted tail)       final answers)      pruned ranges)
          │                   │                   │
          └───────────────────┼───────────────────┘
                              ▼
[ Micro-Forensics ]     deja inspect <id> --view timeline --cursor seq:N --filter "<word>"
                              │  (exact turns, tool arguments, errors)
                              ▼
[ Side-Effects ]        deja inspect <id> --view resources --fields files
                              (exact files touched, commands executed, or URLs accessed)
```

---

## 🛠️ View Selector: When to use which

| View | Purpose | What is included | When to use |
| :--- | :--- | :--- | :--- |
| **`state`** | **Agent Continuation & Handoff** | Milestones (0..LastCompacted) + Live Tail (LastCompacted..EOF) | **Default for continuing work**: 100% state coverage, zero redundancy, ~100x token reduction. |
| **`spine`** | **Human Review & Intent Audit** | Pure User prompts + Final Assistant answers per turn | **To understand human agreements**: 100% ground truth, zero tool noise, zero reasoning tokens. |
| **`compactions`** | **Milestone Overview** | All `compaction/summary` blocks with sequence ranges | To scan major architectural pivots across long-running sessions. |
| **`session`** | **Triage Passport** | Metadata, turn count, event counts, available drill-downs | First glance at a session ID from search results. |
| **`timeline`** | **Microscope / Execution Trace** | Chronological event stream (`--cursor`, `--filter`, `--role`, `--tool`) | Deep debugging of a specific error, prompt, or tool output. |
| **`resources`** | **Files & Commands Audit** | Aggregated tools, shell commands, touched files (`--fields files`) | To see what files were modified or what commands ran. |
| **`lineage`** | **Delegation Tree** | Root session, parent origin, and child subagents | Tracing multi-agent swarms and forked branches. |

---

## 💻 CLI Commands & Examples

### 1. State Handoff (`--view state` or `digest`)
The optimal representation for feeding prior context to a new agent or catching up instantly:
```bash
deja inspect <id> --view state
deja inspect <id> --view state --json
```

### 2. Dialogue Spine (`--view spine` or `dialogue`)
The pure human-AI dialogue without tool spam:
```bash
deja inspect <id> --view spine
deja inspect <id> --view spine --filter "nginx"    # filter dialogue turns by keyword
```

### 3. Milestone Compactions (`--view compactions` or `summaries`)
```bash
deja inspect <id> --view compactions
deja inspect <id> --view compactions --filter "FreeKassa"
```

### 4. Timeline Forensics (`--view timeline`)
```bash
deja inspect <id> --view timeline --limit 20
deja inspect <id> --view timeline --cursor seq:103      # jump to sequence 103 (or --offset 103)
deja inspect <id> --view timeline --role user           # human prompts only
deja inspect <id> --view timeline --tool bash           # bash calls only
deja inspect <id> --view timeline --filter "error"      # search events
```

### 5. Resources & Field Masks (`--view resources`)
```bash
deja inspect <id> --view resources
deja inspect <id> --view resources --fields files       # files touched only (saves thousands of tokens!)
deja inspect <id> --view resources --fields commands    # commands only
```

---

## 🤖 MCP Invocation Examples

```json
// 1. Get the zero-redundancy state continuation
{ "mode": "inspect", "target": "299811f0", "view": "state" }

// 2. Read pure dialogue spine
{ "mode": "inspect", "target": "299811f0", "view": "spine" }

// 3. Inspect milestones matching a keyword
{ "mode": "inspect", "target": "37dcf784", "view": "compactions", "filter": "Beads" }

// 4. Zoom into timeline around error with pagination
{ "mode": "inspect", "target": "299811f0", "view": "timeline", "cursor": "seq:103", "limit": 10 }

// 5. Get touched files only
{ "mode": "inspect", "target": "299811f0", "view": "resources", "fields": "files" }
```

---

## ⚠️ Agent Rules & Invariants

1. **Bare IDs & Prefixes are supported:** `cf71b81f-...`, `299811f0`, `deepseek:<id>`, or `deja://session/<id>` all resolve automatically.
2. **Prefer `--view state` over raw dumps:** When resuming or investigating an earlier session, **always** call `--view state` first. It gives you the full historical milestones plus the active live tail in ~1.5k tokens.
3. **Use `--fields files`:** Do not dump hundreds of shell commands into context when you only need to know which files were touched.
4. **Graceful cross-harness fallback:** Works on DeepSeek (native event sourcing), OpenCode, Claude, Cursor, and Antigravity (indexed projection).
