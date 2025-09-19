package crud

import (
	"context"
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
)

// UserUTXO defines the interface for UserUTXO operations.
type UserUTXO interface {
	Read() UserUTXOReader
	Create(ctx context.Context, utxo *entity.UserUTXO) error
	Update(ctx context.Context, spec *entity.UserUTXOUpdateSpecification) error
}

// UserUTXOReadOperations defines read operations for UserUTXO queries.
type UserUTXOReadOperations interface {
	Find(ctx context.Context) ([]*entity.UserUTXO, error)
	Count(ctx context.Context) (int64, error)
}

// UserUTXOReader provides a fluent builder for querying UserUTXO records.
type UserUTXOReader interface {
	UserUTXOReadOperations

	UserID(userID int) UserUTXOReadOperations
	OutputID() NumericCondition[UserUTXOReader, uint]
	BasketName() StringCondition[UserUTXOReader]
	Status() StringEnumCondition[UserUTXOReader, wdk.UTXOStatus]
	Satoshis() NumericCondition[UserUTXOReader, uint64]
	EstimatedInputSize() NumericCondition[UserUTXOReader, uint64]
	ReservedByID() NumericCondition[UserUTXOReader, uint]
	Paged(limit, offset int, desc bool) UserUTXOReader
	Since(value time.Time, column entity.SinceField) UserUTXOReader
}

type userUtxoRepo interface {
	Add(ctx context.Context, utxo *entity.UserUTXO) error
	Update(ctx context.Context, spec *entity.UserUTXOUpdateSpecification) error
	Find(ctx context.Context, spec *entity.UserUTXOReadSpecification, opts ...queryopts.Options) ([]*entity.UserUTXO, error)
	Count(ctx context.Context, spec *entity.UserUTXOReadSpecification, opts ...queryopts.Options) (int64, error)
}

type userUtxo struct {
	repo           userUtxoRepo
	spec           entity.UserUTXOReadSpecification
	pagingAndSince pagingAndSinceParams
}

// NewUserUTXO creates a new UserUTXO instance.
func NewUserUTXO(repo userUtxoRepo) UserUTXO {
	return &userUtxo{repo: repo}
}

func (u *userUtxo) Read() UserUTXOReader {
	return u
}

func (u *userUtxo) Create(ctx context.Context, utxo *entity.UserUTXO) error {
	if utxo == nil {
		return fmt.Errorf("utxo cannot be nil")
	}
	if err := u.repo.Add(ctx, utxo); err != nil {
		return fmt.Errorf("failed to create utxo: %w", err)
	}
	return nil
}

func (u *userUtxo) Update(ctx context.Context, spec *entity.UserUTXOUpdateSpecification) error {
	if spec == nil {
		return fmt.Errorf("update specification cannot be nil")
	}
	if err := u.repo.Update(ctx, spec); err != nil {
		return fmt.Errorf("failed to update utxo: %w", err)
	}
	return nil
}

func (u *userUtxo) Find(ctx context.Context) ([]*entity.UserUTXO, error) {
	userUTXOs, err := u.repo.Find(ctx, &u.spec,
		append(u.pagingAndSince.Since(), u.pagingAndSince.Paging()...)...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find utxos: %w", err)
	}
	return userUTXOs, nil
}

func (u *userUtxo) Count(ctx context.Context) (int64, error) {
	count, err := u.repo.Count(ctx, &u.spec,
		append(u.pagingAndSince.Since(), u.pagingAndSince.Paging()...)...,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to count utxos: %w", err)
	}
	return count, nil
}

func (u *userUtxo) UserID(userID int) UserUTXOReadOperations {
	u.spec.UserID = to.Ptr(userID)
	return u
}

func (u *userUtxo) OutputID() NumericCondition[UserUTXOReader, uint] {
	return &numericCondition[UserUTXOReader, uint]{
		parent: u,
		conditionSetter: func(spec *entity.Comparable[uint]) {
			u.spec.OutputID = spec
		},
	}
}

func (u *userUtxo) BasketName() StringCondition[UserUTXOReader] {
	return &stringCondition[UserUTXOReader]{
		parent: u,
		conditionSetter: func(spec *entity.Comparable[string]) {
			u.spec.BasketName = spec
		},
	}
}

func (u *userUtxo) Status() StringEnumCondition[UserUTXOReader, wdk.UTXOStatus] {
	return &stringEnumCondition[UserUTXOReader, wdk.UTXOStatus]{
		parent: u,
		conditionSetter: func(spec *entity.Comparable[wdk.UTXOStatus]) {
			u.spec.Status = spec
		},
	}
}

func (u *userUtxo) Satoshis() NumericCondition[UserUTXOReader, uint64] {
	return &numericCondition[UserUTXOReader, uint64]{
		parent: u,
		conditionSetter: func(spec *entity.Comparable[uint64]) {
			u.spec.Satoshis = spec
		},
	}
}

func (u *userUtxo) EstimatedInputSize() NumericCondition[UserUTXOReader, uint64] {
	return &numericCondition[UserUTXOReader, uint64]{
		parent: u,
		conditionSetter: func(spec *entity.Comparable[uint64]) {
			u.spec.EstimatedInputSize = spec
		},
	}
}

func (u *userUtxo) ReservedByID() NumericCondition[UserUTXOReader, uint] {
	return &numericCondition[UserUTXOReader, uint]{
		parent: u,
		conditionSetter: func(spec *entity.Comparable[uint]) {
			u.spec.ReservedByID = spec
		},
	}
}

func (u *userUtxo) Paged(limit, offset int, desc bool) UserUTXOReader {
	u.pagingAndSince.paging = &queryopts.Paging{
		Limit:  limit,
		Offset: offset,
		SortBy: "output_id",
		Sort:   to.IfThen(desc, "DESC").ElseThen("ASC"),
	}
	return u
}

func (u *userUtxo) Since(value time.Time, column entity.SinceField) UserUTXOReader {
	u.pagingAndSince.since = &queryopts.Since{
		Time:  value,
		Field: to.IfThen(column == entity.SinceFieldCreatedAt, "created_at").ElseThen("updated_at"),
	}
	return u
}
