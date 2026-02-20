# Go Interface Contracts: TianjiLLM-Go Migration

**Date**: 2026-02-16

## Core Interfaces

### Provider (EXISTS — no change needed)

```go
type Provider interface {
    TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, params *model.TianjiLLMParams) (*http.Request, error)
    TransformResponse(ctx context.Context, resp *http.Response) (*model.ModelResponse, error)
    TransformStreamChunk(line []byte) (*model.StreamChunk, error)
    GetSupportedParams() []string
    MapParams(params map[string]interface{}) map[string]interface{}
    GetRequestURL(model string, params *model.TianjiLLMParams) string
    SetupHeaders(req *http.Request, params *model.TianjiLLMParams)
}
```

### Callback (NEW — Phase 3)

```go
type Callback interface {
    Name() string
    PreCall(ctx context.Context, data *CallbackData) error
    PostCallSuccess(ctx context.Context, data *CallbackData, response *model.ModelResponse) error
    PostCallFailure(ctx context.Context, data *CallbackData, err error) error
    StreamEvent(ctx context.Context, data *CallbackData, chunk *model.StreamChunk) error
}

type CallbackData struct {
    RequestID    string
    Model        string
    Provider     string
    APIKey       string // hash only
    UserID       string
    TeamID       string
    OrgID        string
    Tags         []string
    StartTime    time.Time
    EndTime      time.Time
    TokensUsed   TokenUsage
    Spend        float64
    CacheHit     bool
    Metadata     map[string]interface{}
}

type TokenUsage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
}
```

### Guardrail (NEW — Phase 4, extends Callback)

```go
type Guardrail interface {
    Callback // embeds all callback methods
    GuardrailName() string
    SupportedHooks() []GuardrailHook
    DefaultOn() bool
    ShouldRun(data *CallbackData, hook GuardrailHook) bool
    Apply(ctx context.Context, inputs *GuardrailInput, inputType InputType) (*GuardrailInput, error)
}

type GuardrailHook string
const (
    HookPreCall  GuardrailHook = "pre_call"
    HookPostCall GuardrailHook = "post_call"
    HookDuring   GuardrailHook = "during_call"
)

type InputType string
const (
    InputTypeRequest  InputType = "request"
    InputTypeResponse InputType = "response"
)

type GuardrailInput struct {
    Messages []model.Message
    Text     string
}
```

### Strategy (EXISTS — needs expansion)

```go
type Strategy interface {
    Pick(deployments []*Deployment) *Deployment
}

// New strategies to implement:
// - LatencyStrategy: picks lowest EMA latency (exists — complete)
// - CostStrategy: picks cheapest per model pricing
// - UsageStrategy: picks lowest current TPM/RPM utilization
// - TagStrategy: filters by deployment tags before delegating
```

### Cache (EXISTS — no change needed)

```go
type Cache interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    MGet(ctx context.Context, keys ...string) ([][]byte, error)
}
```

## Callback Registry (NEW — Phase 3)

```go
type CallbackRegistry struct {
    callbacks  []Callback
    guardrails []Guardrail
}

func (r *CallbackRegistry) Register(cb Callback)
func (r *CallbackRegistry) RegisterGuardrail(g Guardrail)
func (r *CallbackRegistry) FirePreCall(ctx context.Context, data *CallbackData) error
func (r *CallbackRegistry) FirePostCallSuccess(ctx context.Context, data *CallbackData, resp *model.ModelResponse) error
func (r *CallbackRegistry) FirePostCallFailure(ctx context.Context, data *CallbackData, err error) error
func (r *CallbackRegistry) RunGuardrails(ctx context.Context, data *CallbackData, hook GuardrailHook, input *GuardrailInput, inputType InputType) (*GuardrailInput, error)
```
