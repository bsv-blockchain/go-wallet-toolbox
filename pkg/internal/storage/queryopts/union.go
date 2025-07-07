package queryopts

type Options struct {
	Page  *Paging
	Since *Since
	Filters []*Filter
}

func WithPage(page Paging) Options {
	return Options{
		Page: &page,
	}
}

func WithSince(since Since) Options {
	return Options{
		Since: &since,
	}
}

func WithFilters(filter ...*Filter) Options {
	return Options{
		Filters: filter,
	}
}

func ModifyOptions(opts []Options, modifyFunc func(*Options)) {
	for i := range opts {
		modifyFunc(&opts[i])
	}
}

func MergeOptions(opts []Options) Options {
	if len(opts) == 0 {
		return Options{}
	}

	result := Options{}
	for _, opt := range opts {
		if opt.Page != nil {
			result.Page = opt.Page
		}
		if opt.Since != nil {
			result.Since = opt.Since
		}
		if opt.Filters != nil {
			result.Filters = append(result.Filters, opt.Filters...)
		}
	}

	return result
}
