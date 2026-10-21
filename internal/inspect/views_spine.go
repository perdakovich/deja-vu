package inspect

import (
	"encoding/json"





	"strings"
	"time"






)

func projectSpine(trace *DeepSeekTrace, filter string) *SpineView {
	filterLow := strings.ToLower(strings.TrimSpace(filter))

	type turnAccumulator struct {
		turnNumber int
		time       time.Time
		userPrompt string
		userSeq    int
		asstAnswer string
		asstSeq    int
		toolCalls  int
	}

	turnsMap := make(map[int]*turnAccumulator)
	var orderedTurnIDs []int

	getTurn := func(turnNum int) *turnAccumulator {
		if t, ok := turnsMap[turnNum]; ok {
			return t
		}
		t := &turnAccumulator{turnNumber: turnNum}
		turnsMap[turnNum] = t
		orderedTurnIDs = append(orderedTurnIDs, turnNum)
		return t
	}

	currentTurn := 0
	for _, ev := range trace.Events {
		var common struct {
			Turn *int `json:"turn"`
		}
		_ = json.Unmarshal(ev.Data, &common)
		if common.Turn != nil {
			currentTurn = *common.Turn
		}

		t := getTurn(currentTurn)
		if t.time.IsZero() && !ev.Time.IsZero() {
			t.time = ev.Time
		}

		switch ev.Type {
		case "user/message":
			text, isUser := extractUserPromptText(ev.Data)
			if isUser && text != "" {
				t.userPrompt = text
				t.userSeq = ev.Seq
			}
		case "tool/call":
			t.toolCalls++
		case "assistant/message":
			var md struct {
				Message json.RawMessage `json:"message"`
				Content json.RawMessage `json:"content"`
			}
			_ = json.Unmarshal(ev.Data, &md)
			msgPayload := md.Message
			if len(msgPayload) == 0 {
				msgPayload = ev.Data
			}
			text := extractMessageText(msgPayload, md.Content)
			if text != "" {
				t.asstAnswer = text
				t.asstSeq = ev.Seq
			}
		}
	}

	var spineTurns []SpineTurn
	for _, id := range orderedTurnIDs {
		t := turnsMap[id]
		if t.userPrompt == "" && t.asstAnswer == "" {
			continue
		}
		if filterLow != "" {
			uMatch := strings.Contains(strings.ToLower(t.userPrompt), filterLow)
			aMatch := strings.Contains(strings.ToLower(t.asstAnswer), filterLow)
			if !uMatch && !aMatch {
				continue
			}
		}
		// The pointer keeps an unset time out of the JSON rather than
		// writing year 1 for a turn no event stamped.
		var turnTime *time.Time
		if !t.time.IsZero() {
			stamped := t.time
			turnTime = &stamped
		}
		spineTurns = append(spineTurns, SpineTurn{
			Turn:            t.turnNumber,
			Time:            turnTime,
			UserPrompt:      t.userPrompt,
			UserSeq:         t.userSeq,
			AssistantAnswer: t.asstAnswer,
			AssistantSeq:    t.asstSeq,
			ToolCallCount:   t.toolCalls,
		})
	}

	if spineTurns == nil {
		spineTurns = make([]SpineTurn, 0)
	}

	return &SpineView{
		SessionID:  trace.Header.ID,
		TotalTurns: len(spineTurns),
		Turns:      spineTurns,
	}
}

