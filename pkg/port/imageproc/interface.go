package imageproc

import "context"

// AnalyzeInput carries image references and optional caller hints.
type AnalyzeInput struct {
	FilePath string
	URL      string
	Caption  string
	Tags     []string
	OCRText  string
}

// AnalyzeOutput contains derived semantic fields for an image.
type AnalyzeOutput struct {
	Caption string
	Tags    []string
	OCRText string
}

// Analyzer extracts lightweight text semantics from images.
type Analyzer interface {
	Analyze(ctx context.Context, input AnalyzeInput) (*AnalyzeOutput, error)
}
