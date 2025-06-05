package queryopts

import "strings"

type Paging struct {
	Limit  int
	Offset int
	Sort   string
	SortBy string
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
