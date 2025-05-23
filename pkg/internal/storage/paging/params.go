package paging

import "strings"

type Page struct {
	Limit  int
	Offset int
	Sort   string
	SortBy string
}

// ApplyDefaults sets default values for a Page object (in place).
func (p *Page) ApplyDefaults() {
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

func (p *Page) Next() {
	p.Offset += p.Limit
}
