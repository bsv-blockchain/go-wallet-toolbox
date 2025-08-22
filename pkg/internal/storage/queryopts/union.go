package queryopts

type Options struct {
	Page  *Paging
	Since *Since
	Until *Until
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

func WithUntil(until Until) Options {
	return Options{
		Until: &until,
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
		if opt.Until != nil {
			result.Until = opt.Until
		}
	}

	return result
}
