package hook

import (
	"context"
	"fmt"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// mockHook is a test double.
type mockHook struct {
	name      string
	preErr    error
	postErr   error
	preCalls  int
	postCalls int
}

func (m *mockHook) Name() string { return m.name }
func (m *mockHook) PreCall(_ context.Context, _ *model.ChatCompletionRequest) error {
	m.preCalls++
	return m.preErr
}
func (m *mockHook) PostCall(_ context.Context, _ *model.ChatCompletionRequest, _ *model.ModelResponse) error {
	m.postCalls++
	return m.postErr
}

func TestRegistryNewAndLen(t *testing.T) {
	r := NewRegistry()
	if r.Len() != 0 {
		t.Fatalf("expected 0, got %d", r.Len())
	}
	r.Register(&mockHook{name: "a"})
	if r.Len() != 1 {
		t.Fatalf("expected 1, got %d", r.Len())
	}
}

func TestRegistryRunPreCall(t *testing.T) {
	r := NewRegistry()
	h1 := &mockHook{name: "h1"}
	h2 := &mockHook{name: "h2"}
	r.Register(h1)
	r.Register(h2)

	err := r.RunPreCall(context.Background(), &model.ChatCompletionRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h1.preCalls != 1 {
		t.Fatalf("h1 preCalls: got %d, want 1", h1.preCalls)
	}
	if h2.preCalls != 1 {
		t.Fatalf("h2 preCalls: got %d, want 1", h2.preCalls)
	}
}

func TestRegistryRunPreCallShortCircuit(t *testing.T) {
	r := NewRegistry()
	h1 := &mockHook{name: "h1", preErr: fmt.Errorf("blocked")}
	h2 := &mockHook{name: "h2"}
	r.Register(h1)
	r.Register(h2)

	err := r.RunPreCall(context.Background(), &model.ChatCompletionRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if h2.preCalls != 0 {
		t.Fatalf("h2 should not have been called, got %d", h2.preCalls)
	}
}

func TestRegistryRunPostCall(t *testing.T) {
	r := NewRegistry()
	h1 := &mockHook{name: "h1"}
	r.Register(h1)

	err := r.RunPostCall(context.Background(), &model.ChatCompletionRequest{}, &model.ModelResponse{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h1.postCalls != 1 {
		t.Fatalf("h1 postCalls: got %d, want 1", h1.postCalls)
	}
}

func TestRegistryRunPostCallShortCircuit(t *testing.T) {
	r := NewRegistry()
	h1 := &mockHook{name: "h1", postErr: fmt.Errorf("fail")}
	h2 := &mockHook{name: "h2"}
	r.Register(h1)
	r.Register(h2)

	err := r.RunPostCall(context.Background(), &model.ChatCompletionRequest{}, &model.ModelResponse{})
	if err == nil {
		t.Fatal("expected error")
	}
	if h2.postCalls != 0 {
		t.Fatalf("h2 should not have been called")
	}
}

func TestCreateUnknownHook(t *testing.T) {
	_, err := Create("nonexistent_hook_xyz", nil)
	if err == nil {
		t.Fatal("expected error for unknown hook")
	}
}

func TestCreateBannedKeywords(t *testing.T) {
	h, err := Create("banned_keywords", map[string]any{
		"keywords": []any{"bad", "word"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.Name() != "banned_keywords" {
		t.Fatalf("got name %q, want banned_keywords", h.Name())
	}
}

func TestCreateBannedKeywordsNoKeywords(t *testing.T) {
	_, err := Create("banned_keywords", map[string]any{})
	if err == nil {
		t.Fatal("expected error when no keywords configured")
	}
}

func TestBannedKeywordsPreCall(t *testing.T) {
	h, err := Create("banned_keywords", map[string]any{
		"keywords": []any{"forbidden"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Should block
	req := &model.ChatCompletionRequest{
		Messages: []model.Message{
			{Role: "user", Content: "this is FORBIDDEN content"},
		},
	}
	if err := h.PreCall(context.Background(), req); err == nil {
		t.Fatal("expected error for banned keyword")
	}

	// Should pass
	req2 := &model.ChatCompletionRequest{
		Messages: []model.Message{
			{Role: "user", Content: "this is fine"},
		},
	}
	if err := h.PreCall(context.Background(), req2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBannedKeywordsPostCallNoop(t *testing.T) {
	h, err := Create("banned_keywords", map[string]any{
		"keywords": []any{"x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.PostCall(context.Background(), nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBannedKeywordsNonStringContent(t *testing.T) {
	h, err := Create("banned_keywords", map[string]any{
		"keywords": []any{"bad"},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := &model.ChatCompletionRequest{
		Messages: []model.Message{
			{Role: "user", Content: 123}, // non-string content
		},
	}
	if err := h.PreCall(context.Background(), req); err != nil {
		t.Fatalf("unexpected error for non-string content: %v", err)
	}
}

func TestBlockedUserPreCall(t *testing.T) {
	h, err := Create("blocked_user_list", map[string]any{
		"users": []any{"baduser"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if h.Name() != "blocked_user_list" {
		t.Fatalf("got name %q", h.Name())
	}

	// Block via context
	ctx := context.WithValue(context.Background(), "user_id", "baduser") //nolint:staticcheck
	err = h.PreCall(ctx, &model.ChatCompletionRequest{})
	if err == nil {
		t.Fatal("expected blocked error")
	}

	// Allow non-blocked user
	ctx2 := context.WithValue(context.Background(), "user_id", "gooduser") //nolint:staticcheck
	err = h.PreCall(ctx2, &model.ChatCompletionRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Block via req.User field
	user := "baduser"
	err = h.PreCall(context.Background(), &model.ChatCompletionRequest{User: &user})
	if err == nil {
		t.Fatal("expected blocked error via req.User")
	}
}

func TestBlockedUserPostCallNoop(t *testing.T) {
	h, err := Create("blocked_user_list", map[string]any{
		"users": []any{"x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.PostCall(context.Background(), nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistryFromConfig(t *testing.T) {
	reg, err := RegistryFromConfig(map[string]map[string]any{
		"banned_keywords": {"keywords": []any{"test"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if reg.Len() != 1 {
		t.Fatalf("expected 1 hook, got %d", reg.Len())
	}
}

func TestRegistryFromConfigUnknown(t *testing.T) {
	_, err := RegistryFromConfig(map[string]map[string]any{
		"unknown_xyz": {},
	})
	if err == nil {
		t.Fatal("expected error for unknown hook")
	}
}

func TestManagementEventDispatcherNoWebhook(t *testing.T) {
	d := NewManagementEventDispatcher("")
	// Should just log, not panic
	d.Dispatch(context.Background(), ManagementEvent{
		EventType: "test",
		ObjectID:  "123",
	})
}

func TestManagementEventDispatcherWithWebhook(t *testing.T) {
	d := NewManagementEventDispatcher("http://localhost:9999/webhook")
	// Should not block or panic (goroutine will fail silently)
	d.Dispatch(context.Background(), ManagementEvent{
		EventType: "key_created",
		ObjectID:  "key-1",
		Payload:   map[string]string{"foo": "bar"},
	})
}
