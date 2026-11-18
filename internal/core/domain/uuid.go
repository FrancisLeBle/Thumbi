package domain

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// NewUUIDv7 genera un identificador único universal versión 7 (RFC 9562),
// ordenado cronológicamente por timestamp Unix en milisegundos para mantener
// la localidad de inserción en índices B-Tree de PostgreSQL.
func NewUUIDv7() uuid.UUID {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.New()
	}
	return id
}

// ParseUUID valida y convierte una cadena UUID textual en uuid.UUID,
// rechazando cadenas vacías, formatos inválidos o el UUID nulo (00000000-...).
func ParseUUID(raw string) (uuid.UUID, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return uuid.Nil, fmt.Errorf("%w: identificador UUID vacío", ErrInvalidInput)
	}
	parsed, err := uuid.Parse(trimmed)
	if err != nil || parsed == uuid.Nil {
		return uuid.Nil, fmt.Errorf("%w: identificador UUID inválido", ErrInvalidInput)
	}
	return parsed, nil
}

// ParseOptionalUUID convierte un puntero opcional de cadena UUID en *uuid.UUID.
func ParseOptionalUUID(raw *string) (*uuid.UUID, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	parsed, err := ParseUUID(*raw)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
