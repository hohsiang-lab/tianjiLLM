package handler

import (
	"context"
	"encoding/json"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/hook"
)

// createAuditLog inserts an audit log entry if store_audit_logs is enabled.
func (h *Handlers) createAuditLog(ctx context.Context, action, tableName, objectID, changedBy, changedByAPIKey string, beforeValue, updatedValues any) {
	if h.DB == nil || h.Config == nil || !h.Config.GeneralSettings.StoreAuditLogs {
		return
	}

	var beforeJSON, updatedJSON []byte
	if beforeValue != nil {
		beforeJSON, _ = json.Marshal(beforeValue)
	}
	if updatedValues != nil {
		updatedJSON, _ = json.Marshal(updatedValues)
	}

	_, _ = h.DB.InsertAuditLog(ctx, db.InsertAuditLogParams{
		ChangedBy:       changedBy,
		ChangedByApiKey: changedByAPIKey,
		Action:          action,
		TableName:       tableName,
		ObjectID:        objectID,
		BeforeValue:     beforeJSON,
		UpdatedValues:   updatedJSON,
	})
}

// dispatchEvent fires a management event if EventDispatcher is configured.
func (h *Handlers) dispatchEvent(ctx context.Context, eventType, objectID string, payload any) {
	if h.EventDispatcher == nil {
		return
	}
	h.EventDispatcher.Dispatch(ctx, hook.ManagementEvent{
		EventType: eventType,
		ObjectID:  objectID,
		Payload:   payload,
	})
}
