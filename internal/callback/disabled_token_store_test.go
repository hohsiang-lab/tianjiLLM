package callback

import "testing"

func TestInMemoryDisabledTokenStore(t *testing.T) {
	s := NewInMemoryDisabledTokenStore()

	if s.IsDisabled("tok-a") {
		t.Fatal("newly created store must not have any disabled tokens")
	}

	s.Disable("tok-a")
	if !s.IsDisabled("tok-a") {
		t.Fatal("tok-a must be disabled after Disable()")
	}
	if s.IsDisabled("tok-b") {
		t.Fatal("tok-b must not be disabled")
	}

	s.Enable("tok-a")
	if s.IsDisabled("tok-a") {
		t.Fatal("tok-a must not be disabled after Enable()")
	}
}
