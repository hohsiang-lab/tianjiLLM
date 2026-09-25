package model

type Backend string

const (
	BackendDirectOpenAIHTTP Backend = "direct_openai_http"
	BackendChatGPTCodex     Backend = "chatgpt_codex_backend"
)

type CapabilityKey struct {
	Backend Backend
	Model   string
}

type CapabilityRecord struct {
	SupportsStream                    bool
	SupportsNonStream                 bool
	SupportsResponseFormat            bool
	SupportsJSONObject                bool
	SupportsJSONSchema                bool
	SupportsTools                     bool
	SupportsToolChoice                bool
	SupportsTemperature               bool
	SupportsTopP                      bool
	SupportsMaxTokens                 bool
	SupportsMaxCompletionTokens       bool
	SupportsStreamOptionsIncludeUsage bool
	SupportsEmbeddings                bool
	SupportsDimensions                bool
	AllowedDimensions                 []int
}

type CapabilityMatrix map[CapabilityKey]CapabilityRecord

func (m CapabilityMatrix) Lookup(backend Backend, model string) (CapabilityRecord, bool) {
	record, ok := m[CapabilityKey{Backend: backend, Model: model}]
	return record, ok
}
