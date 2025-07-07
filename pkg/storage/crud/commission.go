package crud

import (
	"context"
	"fmt"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"
)

type CommissionOperations interface {
	Find(ctx context.Context) ([]*entity.Commission, error)
}

type Commission interface {
	CommissionOperations

	WithSatoshisGreaterThan(satoshis uint64) Commission
	WithSatoshisLessThan(satoshis uint64) Commission
	WithIsRedeemed(isRedeemed bool) Commission
	WithUserID(userID int) Commission
}

type commissionRepo interface {
	FindCommissions(ctx context.Context, opts ...queryopts.Options) ([]*entity.Commission, error)
}

type commission struct {
	repo commissionRepo
	opts []queryopts.Options
}

func NewCommission(repo commissionRepo) Commission {
	return &commission{
		repo: repo,
		opts: make([]queryopts.Options, 0),
	}
}

func (c *commission) Find(ctx context.Context) ([]*entity.Commission, error) {
	commissions, err := c.repo.FindCommissions(ctx, c.opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to find commissions: %w", err)
	}
	return commissions, nil
}

func (c *commission) WithSatoshisGreaterThan(satoshis uint64) Commission {
	c.opts = append(c.opts, queryopts.WithFilters(&queryopts.Filter{
		Field: entity.CommissionFieldNames.Satoshis,
		Cmp:   queryopts.GreaterThan,
		Value: satoshis,
	}))
	return c
}

func (c *commission) WithSatoshisLessThan(satoshis uint64) Commission {
	c.opts = append(c.opts, queryopts.WithFilters(&queryopts.Filter{
		Field: entity.CommissionFieldNames.Satoshis,
		Cmp:   queryopts.LessThan,
		Value: satoshis,
	}))
	return c
}

func (c *commission) WithIsRedeemed(isRedeemed bool) Commission {
	c.opts = append(c.opts, queryopts.WithFilters(&queryopts.Filter{
		Field: entity.CommissionFieldNames.IsRedeemed,
		Cmp:   queryopts.Equal,
		Value: isRedeemed,
	}))
	return c
}

func (c *commission) WithUserID(userID int) Commission {
	c.opts = append(c.opts, queryopts.WithFilters(&queryopts.Filter{
		Field: entity.CommissionFieldNames.UserID,
		Cmp:   queryopts.Equal,
		Value: userID,
	}))
	return c
}
