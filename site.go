package sitemanager

import (
	"github.com/tinywasm/fmt"
)

type Status uint8

const (
	StatusDraft Status = iota // ZERO VALUE — no publicado
	StatusLive
	StatusSuspended
)

func (s Status) String() string {
	switch s {
	case StatusDraft:
		return "draft"
	case StatusLive:
		return "live"
	case StatusSuspended:
		return "suspended"
	default:
		return "unknown"
	}
}

type transition struct {
	from Status
	to   Status
}

var validTransitions = []transition{
	{from: StatusDraft, to: StatusLive},
	{from: StatusLive, to: StatusSuspended},
	{from: StatusSuspended, to: StatusLive},
}

func ValidateTransition(from, to Status) error {
	if from == to {
		return nil
	}
	for i := 0; i < len(validTransitions); i++ {
		t := validTransitions[i]
		if t.from == from && t.to == to {
			return nil
		}
	}
	return fmt.Errf("site_manager: transicion invalida de %s a %s", from.String(), to.String())
}
