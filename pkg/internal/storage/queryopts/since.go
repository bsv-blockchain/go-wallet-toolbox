package queryopts

import "time"

type Since struct {
	Time      time.Time
	Field     string
	TableName string
}

func (p *Since) ApplyDefaults() {
	if p.Field == "" {
		p.Field = "created_at"
	}
}
