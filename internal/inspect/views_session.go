package inspect

import (


	"path/filepath"








	"github.com/vshulcz/deja-vu/internal/model"


)

func projectSession(trace *DeepSeekTrace, indexFound bool, indexSession model.Session, logPath string) *SessionView {
	project := indexSession.Project
	if project == "" && trace.Header.CWD != "" {
		project = filepath.Base(trace.Header.CWD)
		if project == "/" || project == "." {
			project = "-"
		}
	}

	turnStarts := 0
	turnEnds := 0
	toolCalls := 0
	compactions := 0

	for _, ev := range trace.Events {
		switch ev.Type {
		case "turn/start":
			turnStarts++
		case "turn/end":
			turnEnds++
		case "tool/call":
			toolCalls++
		case "compaction/summary", "compaction/start":
			if ev.Type == "compaction/summary" {
				compactions++
			}
		}
	}

	openTurn := turnStarts > turnEnds

	parent := trace.Header.ParentSession
	if parent == "" && indexSession.Parent != "" {
		parent = indexSession.Parent
	}

	availableViews := []string{"session"}
	if compactions > 0 {
		availableViews = append(availableViews, "compactions")
	}
	availableViews = append(availableViews, "state", "spine", "resources", "timeline", "lineage", "effects")

	return &SessionView{
		ID:              trace.Header.ID,
		Version:         trace.Header.Version,
		CreatedAt:       trace.Header.CreatedAt,
		CWD:             trace.Header.CWD,
		Project:         project,
		ParentSession:   parent,
		IsSeeded:        trace.Header.IsSeeded,
		Origin:          trace.Header.Origin,
		DelegationDepth: trace.Header.DelegationDepth,
		AgentPreset:     trace.Header.AgentPreset,
		LogPath:         logPath,
		CanonicalLog:    trace.Log,
		TotalEvents:     len(trace.Events),
		TurnCount:       turnStarts,
		ToolCallCount:   toolCalls,
		OpenTurn:        openTurn,
		Compactions:     compactions,
		AvailableViews:  availableViews,
	}
}

