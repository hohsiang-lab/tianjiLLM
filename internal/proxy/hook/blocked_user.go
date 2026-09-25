package hook

import (
	"context"
	"fmt"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// BlockedUser rejects requests from users on the block list.
type BlockedUser struct {
	users map[string]struct{}
}

func init() {
	RegisterConstructor("blocked_user_list", func(config map[string]any) (Hook, error) {
		list, _ := config["users"].([]any)
		users := make(map[string]struct{}, len(list))
		for _, v := range list {
			if s, ok := v.(string); ok {
				users[s] = struct{}{}
			}
		}
		return &BlockedUser{users: users}, nil
	})
}

func (b *BlockedUser) Name() string { return "blocked_user_list" }

func (b *BlockedUser) PreCall(ctx context.Context, req *model.ChatCompletionRequest) error {
	userID, _ := ctx.Value("user_id").(string)
	if _, blocked := b.users[userID]; blocked {
		return fmt.Errorf("user is blocked: %s", userID)
	}
	if req.User != nil && *req.User != "" {
		if _, blocked := b.users[*req.User]; blocked {
			return fmt.Errorf("user is blocked: %s", *req.User)
		}
	}
	return nil
}

func (b *BlockedUser) PostCall(_ context.Context, _ *model.ChatCompletionRequest, _ *model.ModelResponse) error {
	return nil
}
