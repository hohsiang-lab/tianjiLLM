package openaitest

import (
	"io"
	"net/http"
	"net/url"
)

type RecordedRequest struct {
	Method string
	Path   string
	Host   string
	Header http.Header
	Body   []byte
	Form   url.Values
}

func recordRequest(r *http.Request) RecordedRequest {
	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()
	form, _ := url.ParseQuery(string(body))
	return RecordedRequest{
		Method: r.Method,
		Path:   r.URL.Path,
		Host:   r.Host,
		Header: r.Header.Clone(),
		Body:   append([]byte(nil), body...),
		Form:   cloneValues(form),
	}
}

func cloneValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for key, vals := range values {
		cloned[key] = append([]string(nil), vals...)
	}
	return cloned
}
