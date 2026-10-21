package inspect

import (
	"time"
)

// Options specifies what session and view to inspect.
type Options struct {
	Target     string `json:"target"`
	View       string `json:"view,omitempty"`
	Cursor     string `json:"cursor,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Filter     string `json:"filter,omitempty"` // text search inside timeline, resources or compactions
	Role       string `json:"role,omitempty"`   // role filter (user, assistant, tool, system)
	Tool       string `json:"tool,omitempty"`   // tool filter (bash, read, edit, etc.)
	Fields     string `json:"fields,omitempty"` // comma-separated field projection (e.g. "files", "commands")
	Activation string `json:"activation,omitempty"`
}

// Completeness status values.
const (
	CompletenessComplete    = "complete"
	CompletenessTruncated   = "truncated"
	CompletenessPartial     = "partial"
	CompletenessUnsupported = "unsupported"
	CompletenessUnknown     = "unknown"
)

// TargetInfo describes the parsed and resolved session target.
type TargetInfo struct {
	Raw     string `json:"raw"`
	Harness string `json:"harness"`
	ID      string `json:"id"`
}

// SourceCoverage records source resolution facts and depth.
type SourceCoverage struct {
	IndexFound   bool   `json:"indexFound"`
	NativeFound  bool   `json:"nativeFound"`
	LogFile      string `json:"logFile,omitempty"`
	Generation   int    `json:"generation,omitempty"`
	TotalEvents  int    `json:"totalEvents"`
	InheritedCut int    `json:"inheritedCut,omitempty"`
}

// Response is the top-level inspect payload. Exactly one view field
// is populated according to the requested View.
type Response struct {
	SchemaVersion  int            `json:"schema_version"`
	Target         TargetInfo     `json:"target"`
	View           string         `json:"view"`
	Completeness   string         `json:"completeness"`
	SourceCoverage SourceCoverage `json:"sourceCoverage"`
	Cursor         string         `json:"cursor,omitempty"`
	NextCursor     string         `json:"nextCursor,omitempty"`
	Notice         string         `json:"notice,omitempty"`
	Session        *SessionView     `json:"session,omitempty"`
	Timeline       *TimelineView    `json:"timeline,omitempty"`
	Lineage        *LineageView     `json:"lineage,omitempty"`
	Resources      *ResourcesView   `json:"resources,omitempty"`
	Effects        *EffectsView     `json:"effects,omitempty"`
	Compactions    *CompactionsView `json:"compactions,omitempty"`
	Spine          *SpineView       `json:"spine,omitempty"`
	State          *StateView       `json:"state,omitempty"`
}

// SessionView carries session-level metadata and lifecycle aggregates.
type SessionView struct {
	ID              string    `json:"id"`
	Version         int       `json:"version"`
	CreatedAt       time.Time `json:"createdAt"`
	CWD             string    `json:"cwd,omitempty"`
	Project         string    `json:"project,omitempty"`
	ParentSession   string    `json:"parentSession,omitempty"`
	IsSeeded        bool      `json:"isSeeded"`
	Origin          string    `json:"origin,omitempty"`
	DelegationDepth int       `json:"delegationDepth"`
	AgentPreset     string    `json:"agentPreset,omitempty"`
	LogPath         string    `json:"logPath"`
	CanonicalLog    string    `json:"canonicalLog"`
	TotalEvents     int       `json:"totalEvents"`
	TurnCount       int       `json:"turnCount"`
	ToolCallCount   int       `json:"toolCallCount"`
	OpenTurn        bool      `json:"openTurn"`
	Compactions     int       `json:"compactions"`
	AvailableViews  []string  `json:"availableViews,omitempty"`
}

// CompactionsView aggregates compaction and milestone summaries.
type CompactionsView struct {
	SessionID   string            `json:"sessionId"`
	TotalCount  int               `json:"totalCount"`
	Compactions []CompactionEntry `json:"compactions"`
}

// CompactionEntry details one compaction checkpoint.
type CompactionEntry struct {
	CompactionID       string    `json:"compactionId"`
	Seq                int       `json:"seq"`
	Time               time.Time `json:"time"`
	Turn               int       `json:"turn,omitempty"`
	ShadowedRange      *[2]int   `json:"shadowedRange,omitempty"`
	ShadowedTokenCount int       `json:"shadowedTokenCount,omitempty"`
	ShadowedSeqs       int       `json:"shadowedSeqs,omitempty"`
	Provider           string    `json:"provider,omitempty"`
	Model              string    `json:"model,omitempty"`
	SummaryText        string    `json:"summary"`
}

// TimelineView carries chronological typed observations.
type TimelineView struct {
	Items      []TimelineItem `json:"items"`
	TotalCount int            `json:"totalCount"`
}

// TimelineItem represents one typed event on the timeline with leaf redaction.
type TimelineItem struct {
	Seq             int             `json:"seq"`
	Type            string          `json:"type"`
	Time            time.Time       `json:"time"`
	Ref             string          `json:"ref"`
	Turn            int             `json:"turn,omitempty"`
	Step            int             `json:"step,omitempty"`
	Role            string          `json:"role,omitempty"`
	Summary         string          `json:"summary,omitempty"`
	Tool            string          `json:"tool,omitempty"`
	CallID          string          `json:"callId,omitempty"`
	SurfaceOp       string          `json:"surfaceOp,omitempty"`
	ReplacesRange   *[2]int         `json:"replacesRange,omitempty"`
	SourceEventSeqs []int           `json:"sourceEventSeqs,omitempty"`
	Compaction      *CompactionFact `json:"compaction,omitempty"`
}

// CompactionFact details compaction lifecycle facts.
type CompactionFact struct {
	CompactionID       string  `json:"compactionId,omitempty"`
	Kind               string  `json:"kind"` // "start", "prune", "summary", "end"
	ShadowedRange      *[2]int `json:"shadowedRange,omitempty"`
	ShadowedTokenCount int     `json:"shadowedTokenCount,omitempty"`
	ShadowedSeqs       []int   `json:"shadowedSeqs,omitempty"`
	Error              string  `json:"error,omitempty"`
}

// LineageView records explicit parental and delegation relationships.
type LineageView struct {
	SessionID       string         `json:"sessionId"`
	ParentSession   string         `json:"parentSession,omitempty"`
	IsSeeded        bool           `json:"isSeeded"`
	Origin          string         `json:"origin,omitempty"`
	DelegationDepth int            `json:"delegationDepth"`
	AgentPreset     string         `json:"agentPreset,omitempty"`
	InheritedPrefix int            `json:"inheritedPrefix,omitempty"`
	Children        []ChildSession `json:"children"`
}

// ChildSession represents a session spawned from this one.
type ChildSession struct {
	ID      string    `json:"id"`
	Harness string    `json:"harness"`
	Project string    `json:"project,omitempty"`
	Title   string    `json:"title,omitempty"`
	Updated time.Time `json:"updated"`
	Kind    string    `json:"kind,omitempty"`
	Agent   string    `json:"agent,omitempty"`
}

// ResourcesView aggregates tools, commands, files, URLs, and errors.
type ResourcesView struct {
	Tools    []ToolUsage   `json:"tools"`
	Commands []string      `json:"commands"`
	Files    []string      `json:"files"`
	URLs     []string      `json:"urls"`
	Errors   []ResourceErr `json:"errors"`
}

// ToolUsage summarizes one tool invocation count.
type ToolUsage struct {
	Name  string `json:"name"`
	Calls int    `json:"calls"`
}

// ResourceErr describes an explicit tool or turn failure.
type ResourceErr struct {
	Seq     int    `json:"seq"`
	Tool    string `json:"tool,omitempty"`
	Ref     string `json:"ref"`
	Message string `json:"message"`
}

// EffectsView presents requested and reported mutation observations.
type EffectsView struct {
	Items      []EffectItem `json:"items"`
	TotalCount int          `json:"totalCount"`
}

// EffectItem details one mutation stage (requested or reported).
type EffectItem struct {
	CallID  string    `json:"callId,omitempty"`
	Tool    string    `json:"tool"`
	Stage   string    `json:"stage"` // "requested" or "reported"
	Seq     int       `json:"seq"`
	Time    time.Time `json:"time"`
	Ref     string    `json:"ref"`
	Target  string    `json:"target,omitempty"`
	Outcome string    `json:"outcome,omitempty"`
	Details string    `json:"details,omitempty"`
}

// SpineView represents the pure dialogue spine (user prompts and final assistant answers)
// with zero tool calls, reasoning blocks, or execution exhaust.
type SpineView struct {
	SessionID  string      `json:"sessionId"`
	TotalTurns int         `json:"totalTurns"`
	Turns      []SpineTurn `json:"turns"`
}

// SpineTurn represents one logical conversation turn.
type SpineTurn struct {
	Turn int `json:"turn"`
	// A pointer, because omitempty does nothing to a struct: a bare
	// time.Time would always be written, and a zero one reads as year 1.
	Time            *time.Time `json:"time,omitempty"`
	UserPrompt      string    `json:"user"`
	UserSeq         int       `json:"userSeq,omitempty"`
	AssistantAnswer string    `json:"assistant"`
	AssistantSeq    int       `json:"assistantSeq,omitempty"`
	ToolCallCount   int       `json:"toolCallCount"`
}

// StateView represents the two-tier state: historical milestone compactions
// plus the uncompacted live tail (dialogue spine after the last compaction).
type StateView struct {
	SessionID      string            `json:"sessionId"`
	Compactions    []CompactionEntry `json:"compactions"`
	TotalCompacted int               `json:"totalCompacted"`
	LiveTail       []SpineTurn       `json:"liveTail"`
	TailStartsSeq  int               `json:"tailStartsSeq"`
	TotalTurns     int               `json:"totalTurns"`
}
