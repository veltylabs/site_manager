package sitemanager

// domainError es el tipo concreto de los errores centinela de este paquete. El código
// los compara haciendo un type assertion y comprobando el valor: == entre dos
// valores de tipo error compila, bajo TinyGo, a runtime.interfaceEqual, lo que incluye
// internal/reflectlite en el binario wasm.
type domainError string

// Error devuelve el texto del error.
//
// Ejemplo: text := ErrNotFound.Error()
func (e domainError) Error() string { return string(e) }

const (
	ErrNotFound          domainError = "site_manager not found"
	ErrAlreadyExists     domainError = "site_manager already exists"
	ErrInvalidTransition domainError = "site_manager invalid transition"
	ErrInvalidData       domainError = "site_manager invalid data"
	ErrNilDependency     domainError = "site_manager nil dependency"
)
