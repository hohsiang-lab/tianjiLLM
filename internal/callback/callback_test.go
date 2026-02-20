package callback

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockLogger struct {
	successCalls int
	failureCalls int
	lastData     LogData
}

func (m *mockLogger) LogSuccess(data LogData) {
	m.successCalls++
	m.lastData = data
}

func (m *mockLogger) LogFailure(data LogData) {
	m.failureCalls++
	m.lastData = data
}

func TestRegistry_LogSuccess(t *testing.T) {
	r := NewRegistry()
	mock1 := &mockLogger{}
	mock2 := &mockLogger{}

	r.Register(mock1)
	r.Register(mock2)
	assert.Equal(t, 2, r.Count())

	data := LogData{
		Model:    "gpt-4o",
		Provider: "openai",
		Latency:  100 * time.Millisecond,
	}

	r.LogSuccess(data)

	assert.Equal(t, 1, mock1.successCalls)
	assert.Equal(t, 1, mock2.successCalls)
	assert.Equal(t, "gpt-4o", mock1.lastData.Model)
}

func TestRegistry_LogFailure(t *testing.T) {
	r := NewRegistry()
	mock := &mockLogger{}

	r.Register(mock)

	data := LogData{
		Model:    "gpt-4o",
		Provider: "openai",
		Error:    assert.AnError,
	}

	r.LogFailure(data)

	assert.Equal(t, 1, mock.failureCalls)
	assert.NotNil(t, mock.lastData.Error)
}

func TestRegistry_Empty(t *testing.T) {
	r := NewRegistry()
	assert.Equal(t, 0, r.Count())

	// Should not panic
	r.LogSuccess(LogData{})
	r.LogFailure(LogData{})
}
