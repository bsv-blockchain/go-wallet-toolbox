package crud

import (
	"context"
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/go-softwarelab/common/pkg/to"
)

// User provides query-building capabilities for retrieving and managing user records.
type User interface {
	Read() UserReader
	Create(ctx context.Context, user *entity.User) error
	Update(ctx context.Context, spec *entity.UserUpdateSpecification) error
}

// UserReadOperations defines read operations for querying User entities.
type UserReadOperations interface {
	Find(ctx context.Context) ([]*entity.User, error)
	Count(ctx context.Context) (int64, error)
}

// UserReader provides a fluent interface for building user queries.
type UserReader interface {
	UserReadOperations

	ID(id int) UserReadOperations
	IdentityKey() StringCondition[UserReader]
	ActiveStorage() StringCondition[UserReader]
	Since(value time.Time, column entity.SinceField) UserReader
	Paged(limit, offset int, desc bool) UserReader
}

type userRepo interface {
	AddUser(ctx context.Context, user *entity.User) error
	UpdateUser(ctx context.Context, spec *entity.UserUpdateSpecification) error
	FindUsers(ctx context.Context, spec *entity.UserReadSpecification, opts ...queryopts.Options) ([]*entity.User, error)
	CountUsers(ctx context.Context, spec *entity.UserReadSpecification, opts ...queryopts.Options) (int64, error)
}

type user struct {
	repo           userRepo
	spec           entity.UserReadSpecification
	pagingAndSince pagingAndSinceParams
}

// NewUser creates a new User query builder instance.
func NewUser(repo userRepo) User {
	return &user{repo: repo}
}

func (u *user) Read() UserReader {
	return u
}

func (u *user) Create(ctx context.Context, user *entity.User) error {
	if user == nil {
		return fmt.Errorf("user cannot be nil")
	}
	if err := u.repo.AddUser(ctx, user); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (u *user) Update(ctx context.Context, spec *entity.UserUpdateSpecification) error {
	if spec == nil {
		return fmt.Errorf("update specification cannot be nil")
	}
	if err := u.repo.UpdateUser(ctx, spec); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

func (u *user) Find(ctx context.Context) ([]*entity.User, error) {
	users, err := u.repo.FindUsers(ctx, &u.spec, u.pagingAndSince.Since()...)
	if err != nil {
		return nil, fmt.Errorf("failed to find users: %w", err)
	}
	return users, nil
}

func (u *user) Count(ctx context.Context) (int64, error) {
	count, err := u.repo.CountUsers(ctx, &u.spec, u.pagingAndSince.Since()...)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}

func (u *user) ID(id int) UserReadOperations {
	u.spec.ID = to.Ptr(id)
	return u
}

func (u *user) IdentityKey() StringCondition[UserReader] {
	return &stringCondition[UserReader]{
		parent: u,
		conditionSetter: func(spec *entity.Comparable[string]) {
			u.spec.IdentityKey = spec
		},
	}
}

func (u *user) ActiveStorage() StringCondition[UserReader] {
	return &stringCondition[UserReader]{
		parent: u,
		conditionSetter: func(spec *entity.Comparable[string]) {
			u.spec.ActiveStorage = spec
		},
	}
}

func (u *user) Since(value time.Time, column entity.SinceField) UserReader {
	u.pagingAndSince.since = &queryopts.Since{
		Time:  value,
		Field: to.IfThen(column == entity.SinceFieldCreatedAt, "created_at").ElseThen("updated_at"),
	}
	return u
}

func (u *user) Paged(limit, offset int, desc bool) UserReader {
	u.pagingAndSince.paging = &queryopts.Paging{
		Limit:  limit,
		Offset: offset,
		SortBy: "id",
		Sort:   to.IfThen(desc, "DESC").ElseThen("ASC"),
	}
	return u
}
