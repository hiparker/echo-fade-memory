package basic

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/hiparker/echo-fade-memory/pkg/port/imageproc"
)

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

// Analyzer is a lightweight fallback image analyzer.
// It uses caller hints, filename/url tokens, and optional sidecar OCR text.
type Analyzer struct{}

func New() *Analyzer {
	return &Analyzer{}
}

func (a *Analyzer) Analyze(ctx context.Context, input imageproc.AnalyzeInput) (*imageproc.AnalyzeOutput, error) {
	_ = ctx
	ocr := strings.TrimSpace(input.OCRText)
	if ocr == "" && input.FilePath != "" {
		ocr = strings.TrimSpace(readSidecarText(input.FilePath + ".ocr.txt"))
	}
	caption := strings.TrimSpace(input.Caption)
	if caption == "" {
		caption = deriveCaption(input.FilePath, input.URL, ocr)
	}
	tags := normalizeTags(append([]string{}, input.Tags...))
	if len(tags) == 0 {
		tags = deriveTags(caption, ocr, input.FilePath, input.URL)
	}
	return &imageproc.AnalyzeOutput{
		Caption: caption,
		Tags:    tags,
		OCRText: ocr,
	}, nil
}

func readSidecarText(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func deriveCaption(filePath, rawURL, ocr string) string {
	name := filePath
	if name == "" {
		name = rawURL
	}
	name = filepath.Base(name)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.TrimSpace(name)
	if name != "" {
		if ocr != "" {
			return name + " image with extracted text"
		}
		return name + " image"
	}
	if ocr != "" {
		return "image containing extracted text"
	}
	return "image asset"
}

func deriveTags(parts ...string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 8)
	for _, part := range parts {
		for _, token := range tokenize(part) {
			if len(token) < 3 {
				continue
			}
			if _, ok := seen[token]; ok {
				continue
			}
			seen[token] = struct{}{}
			out = append(out, token)
			if len(out) >= 12 {
				return out
			}
		}
	}
	sort.Strings(out)
	return out
}

func normalizeTags(tags []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		tag = nonWord.ReplaceAllString(tag, " ")
		tag = strings.TrimSpace(strings.ReplaceAll(tag, "  ", " "))
		if len(tag) < 2 {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}

func tokenize(text string) []string {
	text = strings.ToLower(strings.TrimSpace(text))
	text = strings.ReplaceAll(text, "/", " ")
	text = strings.ReplaceAll(text, ".", " ")
	text = nonWord.ReplaceAllString(text, " ")
	return strings.Fields(text)
}
