package repo

import (
	"context"
	"fmt"
)

type dbTransactionCreator[T any] interface {
	DBTransaction(ctx context.Context, txFunc func(child any) error) error
}

func AsDBTransaction[T dbTransactionCreator[T]](ctx context.Context, parent T, txFunc func(child T) error) error {
	err := parent.DBTransaction(ctx, func(child any) error {
		return txFunc(child.(T))
	})

	if err != nil {
		return fmt.Errorf("DB transaction failed: %w", err)
	}

	return nil
}

type DBTransaction interface {
	Connection() any
}
