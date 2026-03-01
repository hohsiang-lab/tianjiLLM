package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserExtHandlers_NoDB(t *testing.T) {
	h := newTestHandlers()
	for _, tc := range []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
	}{
		{"UserInfo", h.UserInfo},
		{"UserUpdate", h.UserUpdate},
		{"UserDailyActivity", h.UserDailyActivity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.UserInfo(w, httptest.NewRequest(http.MethodGet, "/", nil))
			if w.Code == http.StatusOK {
				t.Fatalf("expected non-200 with nil DB")
			}
		})
	}
}

func TestPromptHandlers_NoDB(t *testing.T) {
	h := newTestHandlers()
	for _, tc := range []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
	}{
		{"PromptCreate", h.PromptCreate},
		{"PromptGet", h.PromptGet},
		{"PromptList", h.PromptList},
		{"PromptUpdate", h.PromptUpdate},
		{"PromptDelete", h.PromptDelete},
		{"PromptVersions", h.PromptVersions},
		{"PromptTest", h.PromptTest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tc.fn(w, httptest.NewRequest(http.MethodGet, "/", nil))
			if w.Code == http.StatusOK {
				t.Fatalf("%s: expected non-200 with nil DB", tc.name)
			}
		})
	}
}

func TestSkillHandlers_NoDB(t *testing.T) {
	h := newTestHandlers()
	for _, tc := range []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
	}{
		{"SkillCreate", h.SkillCreate},
		{"SkillGet", h.SkillGet},
		{"SkillList", h.SkillList},
		{"SkillDelete", h.SkillDelete},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tc.fn(w, httptest.NewRequest(http.MethodGet, "/", nil))
			if w.Code == http.StatusOK {
				t.Fatalf("%s: expected non-200 with nil DB", tc.name)
			}
		})
	}
}

func TestIPHandlers_NoDB(t *testing.T) {
	h := newTestHandlers()
	for _, tc := range []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
	}{
		{"IPAdd", h.IPAdd},
		{"IPDelete", h.IPDelete},
		{"IPList", h.IPList},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tc.fn(w, httptest.NewRequest(http.MethodGet, "/", nil))
			if w.Code == http.StatusOK {
				t.Fatalf("%s: expected non-200 with nil DB", tc.name)
			}
		})
	}
}
