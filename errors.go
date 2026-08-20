package sitemanager

import (
	"github.com/tinywasm/fmt"
)

var (
	ErrNotFound          = fmt.Err("site_manager", "not", "found")
	ErrAlreadyExists     = fmt.Err("site_manager", "already", "exists")
	ErrInvalidTransition = fmt.Err("site_manager", "invalid", "transition")
	ErrInvalidData       = fmt.Err("site_manager", "invalid", "data")
	ErrNilDependency     = fmt.Err("site_manager", "nil", "dependency")
)
