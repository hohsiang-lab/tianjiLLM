package chatgptcodex

import (
	"net/http"
	"strings"
)

func applyClientVersionHeader(req *http.Request, clientVersion string) {
	if req == nil {
		return
	}
	if clientVersion = strings.TrimSpace(clientVersion); clientVersion != "" {
		req.Header.Set("Version", clientVersion)
	}
}
