package sitemanager

// domainError is the concrete type of this package's sentinel errors. Code
// compares them by asserting this type and comparing the value: == between two
// error values compiles, under TinyGo, to runtime.interfaceEqual, which pulls
// internal/reflectlite into the wasm binary.
type domainError string

func (e domainError) Error() string { return string(e) }

const (
	ErrNotFound          domainError = "site_manager not found"
	ErrAlreadyExists     domainError = "site_manager already exists"
	ErrInvalidTransition domainError = "site_manager invalid transition"
	ErrInvalidData       domainError = "site_manager invalid data"
	ErrNilDependency     domainError = "site_manager nil dependency"
)
