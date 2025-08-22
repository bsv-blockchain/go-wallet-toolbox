package queryopts

import "time"

type Until struct {
	Time      time.Time
	Field     string
	TableName string
}

func (p *Until) ApplyDefaults() {
	if p.Field == "" {
		p.Field = "created_at"
	}
}
