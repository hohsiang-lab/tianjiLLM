package ui

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

const keysPerPage = 20

func (h *UIHandler) handleKeys(w http.ResponseWriter, r *http.Request) {
	data := h.loadKeysPageData(r)
	render(r.Context(), w, pages.KeysPage(data))
}

func (h *UIHandler) handleKeysTable(w http.ResponseWriter, r *http.Request) {
	data := h.loadKeysPageData(r)
	render(r.Context(), w, pages.KeysTablePartial(data))
}

func (h *UIHandler) loadKeysPageData(r *http.Request) pages.KeysPageData {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	search := r.URL.Query().Get("search")

	data := pages.KeysPageData{
		Page:   page,
		Search: search,
	}

	if h.DB == nil {
		return data
	}

	offset := int32((page - 1) * keysPerPage)
	tokens, err := h.DB.ListVerificationTokens(r.Context(), db.ListVerificationTokensParams{
		Limit:  keysPerPage + 1, // fetch one extra to detect next page
		Offset: offset,
	})
	if err != nil {
		return data
	}

	hasMore := len(tokens) > keysPerPage
	if hasMore {
		tokens = tokens[:keysPerPage]
	}

	for _, t := range tokens {
		name := ""
		if t.KeyName != nil {
			name = *t.KeyName
		}
		alias := ""
		if t.KeyAlias != nil {
			alias = *t.KeyAlias
		}
		blocked := false
		if t.Blocked != nil {
			blocked = *t.Blocked
		}

		// Filter by search term
		if search != "" {
			s := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(name), s) &&
				!strings.Contains(strings.ToLower(alias), s) &&
				!strings.Contains(strings.ToLower(t.Token), s) {
				continue
			}
		}

		row := pages.KeyRow{
			Token:     t.Token,
			KeyName:   name,
			KeyAlias:  alias,
			Spend:     t.Spend,
			MaxBudget: t.MaxBudget,
			Models:    t.Models,
			Blocked:   blocked,
		}
		if t.CreatedAt.Valid {
			row.CreatedAt = t.CreatedAt.Time
		}
		data.Keys = append(data.Keys, row)
	}

	// Rough total pages estimation
	if hasMore {
		data.TotalPages = page + 1
	} else {
		data.TotalPages = page
	}

	return data
}

func (h *UIHandler) handleKeyCreate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	keyName := r.FormValue("key_name")
	modelsStr := r.FormValue("models")
	maxBudgetStr := r.FormValue("max_budget")

	// Generate token
	token := generateAPIKey()
	hashBytes := hashKey(token)

	var maxBudget *float64
	if maxBudgetStr != "" {
		v, err := strconv.ParseFloat(maxBudgetStr, 64)
		if err == nil {
			maxBudget = &v
		}
	}

	var models []string
	if modelsStr != "" {
		for _, m := range strings.Split(modelsStr, ",") {
			m = strings.TrimSpace(m)
			if m != "" {
				models = append(models, m)
			}
		}
	}

	params := db.CreateVerificationTokenParams{
		Token:       hashBytes,
		KeyName:     &keyName,
		Spend:       0,
		MaxBudget:   maxBudget,
		Models:      models,
		Permissions: []byte("{}"),
		Metadata:    mustJSON(map[string]string{"generated_by": "ui"}),
	}

	_, err := h.DB.CreateVerificationToken(r.Context(), params)
	if err != nil {
		http.Error(w, "failed to create key: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return updated table
	data := h.loadKeysPageData(r)
	render(r.Context(), w, pages.KeysTablePartial(data))
}

func (h *UIHandler) handleKeyDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	if token == "" {
		http.Error(w, "token required", http.StatusBadRequest)
		return
	}

	_ = h.DB.DeleteVerificationToken(r.Context(), token)

	data := h.loadKeysPageData(r)
	render(r.Context(), w, pages.KeysTablePartial(data))
}

func (h *UIHandler) handleKeyBlock(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	if token == "" {
		http.Error(w, "token required", http.StatusBadRequest)
		return
	}

	_ = h.DB.BlockVerificationToken(r.Context(), token)

	data := h.loadKeysPageData(r)
	render(r.Context(), w, pages.KeysTablePartial(data))
}

func (h *UIHandler) handleKeyUnblock(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	if token == "" {
		http.Error(w, "token required", http.StatusBadRequest)
		return
	}

	_ = h.DB.UnblockVerificationToken(r.Context(), token)

	data := h.loadKeysPageData(r)
	render(r.Context(), w, pages.KeysTablePartial(data))
}

func generateAPIKey() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return "sk-" + hex.EncodeToString(b)
}

func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func mustJSON(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}
