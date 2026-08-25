package queryopts

import "strings"

type Paging struct {
	Limit  int
	Offset int
	Sort   string
	SortBy string
	// ThenBy breaks ties on SortBy. Rows that compare equal on SortBy otherwise come back in
	// whatever order the engine happens to pick, which is unstable between queries: under
	// OFFSET paging that can drop a row from one page and repeat it on the next. Empty leaves
	// the ordering as it was.
	ThenBy string
}

// ApplyDefaults sets default values for a Paging object (in place).
func (p *Paging) ApplyDefaults() {
	if p.Limit <= 0 {
		p.Limit = -1
	}

	p.SortBy = strings.ToLower(p.SortBy)
	if p.SortBy == "" {
		p.SortBy = "created_at"
	}

	p.ThenBy = strings.ToLower(p.ThenBy)

	if strings.ToLower(p.Sort) == "asc" {
		p.Sort = "ASC"
	} else {
		p.Sort = "DESC"
	}
}

func (p *Paging) Next() {
	p.Offset += p.Limit
}

func (p *Paging) IsDesc() bool {
	return strings.ToLower(p.Sort) == "desc"
}
