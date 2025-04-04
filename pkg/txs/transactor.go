package txs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TxManager struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

func NewTxManager(db *pgxpool.Pool, logger *slog.Logger) *TxManager {
	return &TxManager{
		db:     db,
		logger: logger,
	}
}

func injectTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

func (t *TxManager) WithTransaction(ctx context.Context, txFunc func(ctx context.Context) error) error {
	tx, err := t.db.Begin(ctx)
	if err != nil {
		t.logger.Error("Ошибка при начале транзакции", "error", err)
		return fmt.Errorf("ошибка при начале транзакции: %w", err)
	}

	txCtx := injectTx(ctx, tx)

	defer func() {
		if r := recover(); r != nil {
			t.logger.Error("Паника в транзакции, выполняем rollback", "panic", r)

			_ = tx.Rollback(ctx)

			panic(r)
		}
	}()

	if err := txFunc(txCtx); err != nil {
		t.logger.Error("Ошибка в транзакции, выполняем rollback", "error", err)

		if rbErr := tx.Rollback(ctx); rbErr != nil {
			t.logger.Error("Ошибка при rollback транзакции", "error", rbErr)
			return fmt.Errorf("ошибка в транзакции: %w, ошибка rollback: %v", err, rbErr)
		}

		return fmt.Errorf("ошибка в транзакции: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.logger.Error("Ошибка при commit транзакции", "error", err)
		return fmt.Errorf("ошибка при commit транзакции: %w", err)
	}

	return nil
}
