package queryopts

type QueryOptsUnion struct {
	Page  *Paging
	Since *Since
}

func WithPage(page Paging) QueryOptsUnion {
	return QueryOptsUnion{
		Page:  &page,
	}
}

func WithSince(since Since) QueryOptsUnion {
	return QueryOptsUnion{
		Since: &since,
	}
}

func MergeOptions(opts ...QueryOptsUnion) QueryOptsUnion {
	if len(opts) == 0 {
		return QueryOptsUnion{}
	}

	result := QueryOptsUnion{}
	for _, opt := range opts {
		if opt.Page != nil {
			result.Page = opt.Page
		}
		if opt.Since != nil {
			result.Since = opt.Since
		}
	}

	return result
}
