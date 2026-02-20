package guardrail

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// BedrockGuardrail wraps AWS Bedrock Guardrails.
type BedrockGuardrail struct {
	client           *bedrockruntime.Client
	guardrailID      string
	guardrailVersion string
}

func init() {
	defaultRegistry.Register(&BedrockGuardrail{})
}

// NewBedrockGuardrail creates a Bedrock guardrail with the given config.
func NewBedrockGuardrail(guardrailID, guardrailVersion, region string) (*BedrockGuardrail, error) {
	opts := []func(*awsconfig.LoadOptions) error{}
	if region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("bedrock guardrail: aws config: %w", err)
	}

	version := guardrailVersion
	if version == "" {
		version = "DRAFT"
	}

	return &BedrockGuardrail{
		client:           bedrockruntime.NewFromConfig(cfg),
		guardrailID:      guardrailID,
		guardrailVersion: version,
	}, nil
}

func (b *BedrockGuardrail) Name() string { return "bedrock_guardrail" }

func (b *BedrockGuardrail) SupportedHooks() []Hook { return []Hook{HookPreCall, HookPostCall} }

func (b *BedrockGuardrail) Run(ctx context.Context, hook Hook, req *model.ChatCompletionRequest, resp *model.ModelResponse) (Result, error) {
	if b.client == nil {
		return Result{Passed: true}, nil
	}

	content := extractContent(hook, req, resp)

	if content == "" {
		return Result{Passed: true}, nil
	}

	source := types.GuardrailContentSourceInput
	if hook == HookPostCall {
		source = types.GuardrailContentSourceOutput
	}

	out, err := b.client.ApplyGuardrail(ctx, &bedrockruntime.ApplyGuardrailInput{
		GuardrailIdentifier: aws.String(b.guardrailID),
		GuardrailVersion:    aws.String(b.guardrailVersion),
		Source:              source,
		Content: []types.GuardrailContentBlock{
			&types.GuardrailContentBlockMemberText{
				Value: types.GuardrailTextBlock{
					Text: aws.String(content),
				},
			},
		},
	})
	if err != nil {
		return Result{}, fmt.Errorf("bedrock guardrail: %w", err)
	}

	if out.Action == types.GuardrailActionGuardrailIntervened {
		msg := "Content blocked by Bedrock guardrail"
		for _, o := range out.Outputs {
			if o.Text != nil {
				msg = *o.Text
				break
			}
		}
		return Result{Passed: false, Message: msg}, nil
	}

	return Result{Passed: true}, nil
}

// defaultRegistry is used only for init() self-registration.
// The actual registry is created per-server instance.
var defaultRegistry = NewRegistry()
