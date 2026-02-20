package huggingface

import (
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://router.huggingface.co/v1"

type Provider struct{ *openai.Provider }

func init() {
	provider.Register("huggingface", &Provider{openai.NewWithBaseURL(defaultBaseURL)})
}
