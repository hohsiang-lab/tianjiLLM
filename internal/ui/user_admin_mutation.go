package ui

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

const activeAdminMutationLockName = "tianjillm:active-admin-mutation"

var errUserMutationTransactionsUnavailable = errors.New("user mutation transactions unavailable")

type transactionBeginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

func (h *UIHandler) withActiveAdminMutationLock(
	ctx context.Context,
	mutate func(*db.Queries) error,
) error {
	var beginner transactionBeginner = h.Pool
	if h.userMutationTxBeginner != nil {
		beginner = h.userMutationTxBeginner
	}
	if beginner == nil {
		return errUserMutationTransactionsUnavailable
	}

	tx, err := beginner.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(
		ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		activeAdminMutationLockName,
	); err != nil {
		return err
	}

	if err := mutate(h.DB.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
