package openai

import (
	"errors"
	"testing"
)

func TestResolveWebSearchModel_SkipsChatCompletionsSearch(t *testing.T) {
	t.Setenv("OPENAI_WEB_SEARCH_MODEL", "gpt-5-search-api")
	t.Setenv("OPENAI_COMPANY_SEARCH_MODEL", "gpt-4o-mini-search-preview")
	if got := resolveWebSearchModel("gpt-4o-search-preview"); got != defaultWebSearchModel {
		t.Fatalf("got %s want %s", got, defaultWebSearchModel)
	}
	t.Setenv("OPENAI_WEB_SEARCH_MODEL", "gpt-4.1-mini")
	if got := resolveWebSearchModel(""); got != "gpt-4.1-mini" {
		t.Fatalf("got %s", got)
	}
}

func TestIsReasoningModel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		model string
		want  bool
	}{
		{name: "o1", model: "o1-preview", want: true},
		{name: "o3", model: "o3-mini", want: true},
		{name: "o dash", model: "o-4", want: true},
		{name: "gpt4o", model: "gpt-4o-mini", want: false},
		{name: "empty", model: "  ", want: false},
		{name: "case", model: "O1-mini", want: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isReasoningModel(tt.model); got != tt.want {
				t.Fatalf("isReasoningModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestErrorClassifiers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      error
		temp     bool
		fmt      bool
		beta     bool
		notFound bool
	}{
		{
			name: "temperature unsupported",
			err:  errors.New(`Unsupported parameter: 'temperature'`),
			temp: true,
		},
		{
			name: "response_format unsupported",
			err:  errors.New(`response_format is Unsupported for this endpoint`),
			fmt:  true,
		},
		{
			name: "beta limitations",
			err:  errors.New(`beta-limitations: temperature must be 1`),
			beta: true,
		},
		{
			name:     "model not found",
			err:      errors.New(`The model 'foo' does not exist`),
			notFound: true,
		},
		{
			name: "nil err",
			err:  nil,
		},
		{
			name: "unrelated",
			err:  errors.New("rate limit exceeded"),
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isUnsupportedTemperatureErr(tt.err); got != tt.temp {
				t.Errorf("isUnsupportedTemperatureErr = %v, want %v", got, tt.temp)
			}
			if got := isUnsupportedResponseFormatErr(tt.err); got != tt.fmt {
				t.Errorf("isUnsupportedResponseFormatErr = %v, want %v", got, tt.fmt)
			}
			if got := isBetaLimitationsErr(tt.err); got != tt.beta {
				t.Errorf("isBetaLimitationsErr = %v, want %v", got, tt.beta)
			}
			if got := isModelNotFoundErr(tt.err); got != tt.notFound {
				t.Errorf("isModelNotFoundErr = %v, want %v", got, tt.notFound)
			}
		})
	}
}
