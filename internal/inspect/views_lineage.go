package inspect

import (










	"github.com/vshulcz/deja-vu/internal/index"

	"github.com/vshulcz/deja-vu/internal/policy"

)

func projectLineage(trace *DeepSeekTrace, indexDir string, sessionID string, pol policy.Policy, activation string) *LineageView {
	parent := sanitizeString(trace.Header.ParentSession, 160)

	var children []ChildSession
	if indexDir != "" {
		if kids, err := index.ChildrenOf(indexDir, sessionID); err == nil {
			for _, k := range kids {
				// Children are independent records. Reapply both policy gates
				// before exposing even their metadata through the lineage view.
				if k.Harness == "" || blocked(pol, activation, k.Path, k.Project) {
					continue
				}
				children = append(children, ChildSession{
					ID:      sanitizeString(k.ID, 160),
					Harness: sanitizeString(k.Harness, 60),
					Project: sanitizeString(k.Project, 160),
					Title:   sanitizeString(k.Title, 100),
					Updated: k.Updated,
					Kind:    sanitizeString(k.Kind, 80),
					Agent:   sanitizeString(k.Agent, 80),
				})
			}
		}
	}
	if children == nil {
		children = make([]ChildSession, 0)
	}

	if len(children) > maxLineageChildren {
		children = children[:maxLineageChildren]
	}

	inheritedCut := 0
	if trace.Header.IsSeeded {
		inheritedCut = findInheritedCut(trace.Events)
	}

	return &LineageView{
		SessionID:       sanitizeString(trace.Header.ID, 160),
		ParentSession:   parent,
		IsSeeded:        trace.Header.IsSeeded,
		Origin:          sanitizeString(trace.Header.Origin, 80),
		DelegationDepth: trace.Header.DelegationDepth,
		AgentPreset:     sanitizeString(trace.Header.AgentPreset, 100),
		InheritedPrefix: inheritedCut,
		Children:        children,
	}
}

