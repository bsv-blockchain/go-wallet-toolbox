package crud

import "github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"

type pagingAndSinceParams struct {
	since  *queryopts.Since
	paging *queryopts.Paging
}

func (c *pagingAndSinceParams) QueryOpts() []queryopts.Options {
	var opts []queryopts.Options
	opts = append(opts, c.Since()...)
	opts = append(opts, c.Paging()...)

	return opts
}

func (c *pagingAndSinceParams) Since() []queryopts.Options {
	if c.since == nil {
		return nil
	}
	return []queryopts.Options{queryopts.WithSince(*c.since)}
}

func (c *pagingAndSinceParams) Paging() []queryopts.Options {
	if c.paging == nil {
		return nil
	}
	return []queryopts.Options{queryopts.WithPage(*c.paging)}
}
