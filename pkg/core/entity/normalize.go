package entity

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
)

var multiSpace = regexp.MustCompile(`\s+`)

// NormalizeName canonicalizes an entity surface form for storage and matching.
func NormalizeName(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, `"'()[]{}<>`)
	raw = multiSpace.ReplaceAllString(raw, " ")
	return strings.ToLower(raw)
}

// DisplayName keeps a readable version for UI and debugging.
func DisplayName(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, `"'()[]{}<>`)
	return multiSpace.ReplaceAllString(raw, " ")
}

// StableEntityID derives a deterministic entity id from type and name.
func StableEntityID(entityType, canonicalName string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(entityType))
	_, _ = h.Write([]byte{':'})
	_, _ = h.Write([]byte(canonicalName))
	return fmt.Sprintf("ent_%x", h.Sum64())
}

// StableRelationID derives a deterministic relation id from endpoints and type.
func StableRelationID(fromEntityID, toEntityID, relationType string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(fromEntityID))
	_, _ = h.Write([]byte("->"))
	_, _ = h.Write([]byte(toEntityID))
	_, _ = h.Write([]byte{':'})
	_, _ = h.Write([]byte(relationType))
	return fmt.Sprintf("rel_%x", h.Sum64())
}
