package entity

import (
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/hiparker/echo-fade-memory/pkg/core/model"
)

type Extraction struct {
	Entities  []model.Entity
	Aliases   []model.EntityAlias
	Relations []model.Relation
	Links     []model.MemoryEntityLink
}

type candidate struct {
	Name       string
	Type       string
	Confidence float64
	Role       string
}

// ExtractMemoryGraph builds a lightweight KG view from a memory snapshot.
func ExtractMemoryGraph(memory *model.Memory) Extraction {
	if memory == nil {
		return Extraction{}
	}
	seen := map[string]model.Entity{}
	aliases := map[string]model.EntityAlias{}
	links := map[string]model.MemoryEntityLink{}
	now := time.Now()

	for _, c := range collectCandidates(memory) {
		display := DisplayName(c.Name)
		canonical := NormalizeName(display)
		if canonical == "" || len(canonical) < 2 {
			continue
		}
		id := StableEntityID(c.Type, canonical)
		if existing, ok := seen[id]; ok {
			if c.Confidence > existing.Confidence {
				existing.Confidence = c.Confidence
			}
			existing.MemoryCount++
			existing.LastSeenAt = now
			seen[id] = existing
		} else {
			seen[id] = model.Entity{
				ID:            id,
				CanonicalName: canonical,
				DisplayName:   display,
				EntityType:    c.Type,
				Description:   display,
				Confidence:    c.Confidence,
				MemoryCount:   1,
				FirstSeenAt:   now,
				LastSeenAt:    now,
			}
		}
		aliasKey := id + ":" + display
		aliases[aliasKey] = model.EntityAlias{
			EntityID: id,
			Alias:    display,
		}
		linkKey := memory.ID + ":" + id + ":" + c.Role
		links[linkKey] = model.MemoryEntityLink{
			MemoryID:   memory.ID,
			EntityID:   id,
			Role:       c.Role,
			Mention:    display,
			Confidence: c.Confidence,
		}
	}

	entityList := make([]model.Entity, 0, len(seen))
	for _, ent := range seen {
		entityList = append(entityList, ent)
	}
	sort.Slice(entityList, func(i, j int) bool {
		if entityList[i].EntityType == entityList[j].EntityType {
			return entityList[i].CanonicalName < entityList[j].CanonicalName
		}
		return entityList[i].EntityType < entityList[j].EntityType
	})

	aliasList := make([]model.EntityAlias, 0, len(aliases))
	for _, alias := range aliases {
		aliasList = append(aliasList, alias)
	}
	sort.Slice(aliasList, func(i, j int) bool {
		if aliasList[i].EntityID == aliasList[j].EntityID {
			return aliasList[i].Alias < aliasList[j].Alias
		}
		return aliasList[i].EntityID < aliasList[j].EntityID
	})

	linkList := make([]model.MemoryEntityLink, 0, len(links))
	for _, link := range links {
		linkList = append(linkList, link)
	}
	sort.Slice(linkList, func(i, j int) bool {
		if linkList[i].EntityID == linkList[j].EntityID {
			return linkList[i].Role < linkList[j].Role
		}
		return linkList[i].EntityID < linkList[j].EntityID
	})

	relations := buildCooccurrenceRelations(entityList, memory, now)
	return Extraction{
		Entities:  entityList,
		Aliases:   aliasList,
		Relations: relations,
		Links:     linkList,
	}
}

func collectCandidates(memory *model.Memory) []candidate {
	candidates := make([]candidate, 0, 12)
	add := func(name, entityType, role string, confidence float64) {
		if strings.TrimSpace(name) == "" {
			return
		}
		candidates = append(candidates, candidate{
			Name:       name,
			Type:       entityType,
			Confidence: confidence,
			Role:       role,
		})
	}

	if topic := pickTopic(memory); topic != "" {
		add(topic, "topic", "topic", 0.72)
	}
	for _, token := range tokenCandidates(memory.Content + "\n" + memory.Summary + "\n" + memory.ResidualContent) {
		switch classifyToken(token) {
		case "url":
			add(token, "url", "reference", 0.92)
		case "path":
			add(token, "path", "reference", 0.9)
		case "symbol":
			add(token, "symbol", "reference", 0.84)
		}
	}
	for _, ref := range memory.SourceRefs {
		if ref.Ref != "" {
			entityType := classifyToken(ref.Ref)
			if entityType == "" {
				entityType = "source"
			}
			add(ref.Ref, entityType, "source_ref", 0.88)
		}
		if ref.Kind != "" {
			add(ref.Kind, "source_kind", "source_kind", 0.6)
		}
	}
	return candidates
}

func pickTopic(memory *model.Memory) string {
	for _, raw := range []string{memory.Summary, memory.ResidualContent, memory.Content} {
		text := strings.TrimSpace(raw)
		if text == "" {
			continue
		}
		text = strings.ReplaceAll(text, "\n", " ")
		text = strings.TrimSpace(text)
		if len(text) > 96 {
			text = strings.TrimSpace(text[:96])
		}
		return text
	}
	return ""
}

// QueryTerms derives normalized lookup terms for KG-backed recall.
func QueryTerms(text string) []string {
	seen := map[string]struct{}{}
	add := func(term string, out *[]string) {
		term = NormalizeName(term)
		if len(term) < 2 {
			return
		}
		if _, ok := seen[term]; ok {
			return
		}
		seen[term] = struct{}{}
		*out = append(*out, term)
	}

	out := make([]string, 0, 8)
	add(text, &out)
	for _, token := range tokenCandidates(text) {
		add(token, &out)
	}
	return out
}

func tokenCandidates(text string) []string {
	parts := strings.FieldsFunc(text, func(r rune) bool {
		switch {
		case unicode.IsSpace(r):
			return true
		case strings.ContainsRune(`,;()[]{}<>`, r):
			return true
		default:
			return false
		}
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, `"'.,!?`)
		if part == "" || len(part) < 3 {
			continue
		}
		out = append(out, part)
	}
	return out
}

func classifyToken(token string) string {
	token = strings.TrimSpace(token)
	switch {
	case strings.HasPrefix(token, "http://") || strings.HasPrefix(token, "https://"):
		return "url"
	case looksLikePath(token):
		return "path"
	case looksLikeSymbol(token):
		return "symbol"
	default:
		return ""
	}
}

func looksLikePath(token string) bool {
	if strings.Count(token, "/") < 1 {
		return false
	}
	if strings.HasPrefix(token, "http://") || strings.HasPrefix(token, "https://") {
		return false
	}
	return strings.ContainsAny(token, "./")
}

func looksLikeSymbol(token string) bool {
	if strings.Contains(token, "::") || strings.Contains(token, ".") || strings.Contains(token, "_") {
		return true
	}
	hasLetter := false
	hasUpper := false
	for _, r := range token {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsUpper(r) {
			hasUpper = true
		}
	}
	return hasLetter && hasUpper
}

func buildCooccurrenceRelations(entities []model.Entity, memory *model.Memory, now time.Time) []model.Relation {
	if len(entities) < 2 || memory == nil {
		return nil
	}
	relations := make([]model.Relation, 0, len(entities)*2)
	for i := 0; i < len(entities); i++ {
		for j := i + 1; j < len(entities); j++ {
			from := entities[i]
			to := entities[j]
			relations = append(relations, model.Relation{
				ID:             StableRelationID(from.ID, to.ID, "related_to"),
				FromEntityID:   from.ID,
				ToEntityID:     to.ID,
				RelationType:   "related_to",
				Evidence:       memory.Summary,
				SourceMemoryID: memory.ID,
				Weight:         1,
				Confidence:     0.55,
				FirstSeenAt:    now,
				LastSeenAt:     now,
			})
		}
	}
	return relations
}
