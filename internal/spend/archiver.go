package spend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// StorageBackend abstracts writing archived spend logs to cloud storage.
type StorageBackend interface {
	Upload(ctx context.Context, key string, data []byte) (location string, err error)
	Name() string
}

// Archiver exports old spend logs to cloud storage and records the archive.
type Archiver struct {
	DB      *db.Queries
	Storage StorageBackend
	// BatchSize controls how many rows to export per batch. Default 10000.
	BatchSize int32
}

func pgDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

func pgTSTZ(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// Archive exports spend logs in [from, to) to storage, records the archive, and deletes originals.
// Idempotent: checks for overlapping archives before proceeding.
func (a *Archiver) Archive(ctx context.Context, from, to time.Time) error {
	// Idempotency: check for overlapping archives
	existing, err := a.DB.GetSpendArchiveByDateRange(ctx, db.GetSpendArchiveByDateRangeParams{
		DateTo:   pgDate(from),
		DateFrom: pgDate(to),
	})
	if err != nil {
		return fmt.Errorf("check existing archives: %w", err)
	}
	if len(existing) > 0 {
		log.Printf("archiver: skipping [%s, %s) — %d overlapping archive(s) exist",
			from.Format("2006-01-02"), to.Format("2006-01-02"), len(existing))
		return nil
	}

	fromTS := pgTSTZ(from)
	toTS := pgTSTZ(to)

	// Count entries
	count, err := a.DB.CountSpendLogsByDateRange(ctx, db.CountSpendLogsByDateRangeParams{
		Starttime:   fromTS,
		Starttime_2: toTS,
	})
	if err != nil {
		return fmt.Errorf("count spend logs: %w", err)
	}
	if count == 0 {
		log.Printf("archiver: no spend logs in [%s, %s)", from.Format("2006-01-02"), to.Format("2006-01-02"))
		return nil
	}

	batchSize := a.BatchSize
	if batchSize <= 0 {
		batchSize = 10000
	}

	// Export rows
	rows, err := a.DB.GetSpendLogsForArchival(ctx, db.GetSpendLogsForArchivalParams{
		Starttime:   fromTS,
		Starttime_2: toTS,
		Limit:       batchSize,
	})
	if err != nil {
		return fmt.Errorf("fetch spend logs: %w", err)
	}

	var data []byte
	if len(rows) > 1000 {
		var buf bytes.Buffer
		for _, row := range rows {
			line, _ := json.Marshal(row)
			buf.Write(line)
			buf.WriteByte('\n')
		}
		data = buf.Bytes()
	} else {
		data, err = json.Marshal(rows)
		if err != nil {
			return fmt.Errorf("marshal spend logs: %w", err)
		}
	}

	key := fmt.Sprintf("spend-logs/%s_to_%s.json",
		from.Format("2006-01-02"), to.Format("2006-01-02"))

	location, err := a.Storage.Upload(ctx, key, data)
	if err != nil {
		return fmt.Errorf("upload to %s: %w", a.Storage.Name(), err)
	}

	// Record archive
	_, err = a.DB.CreateSpendArchive(ctx, db.CreateSpendArchiveParams{
		DateFrom:        pgDate(from),
		DateTo:          pgDate(to),
		StorageType:     a.Storage.Name(),
		StorageLocation: location,
		EntryCount:      count,
	})
	if err != nil {
		return fmt.Errorf("record archive: %w", err)
	}

	// Delete archived rows
	err = a.DB.DeleteSpendLogsByDateRange(ctx, db.DeleteSpendLogsByDateRangeParams{
		Starttime:   fromTS,
		Starttime_2: toTS,
	})
	if err != nil {
		return fmt.Errorf("delete archived spend logs: %w", err)
	}

	log.Printf("archiver: archived %d spend logs [%s, %s) → %s",
		count, from.Format("2006-01-02"), to.Format("2006-01-02"), location)
	return nil
}
