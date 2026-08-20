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

func ValidateTransition(from, to Status) error {
	if from == to {
		return nil
	}
	valid := false
	if from == StatusDraft && to == StatusLive {
		valid = true
	} else if from == StatusLive && to == StatusSuspended {
		valid = true
	} else if from == StatusSuspended && to == StatusLive {
		valid = true
	}
	if !valid {
		return fmt.Errf("site_manager: transicion invalida de %s a %s", from.String(), to.String())
	}
	return nil
}
