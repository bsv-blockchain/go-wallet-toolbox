package queryopts

type Comparator string

const (
	Equal              Comparator = "="
	NotEqual           Comparator = "<>"
	GreaterThan        Comparator = ">"
	GreaterThanOrEqual Comparator = ">="
	LessThan           Comparator = "<"
	LessThanOrEqual    Comparator = "<="
	Like               Comparator = "LIKE"
	In                 Comparator = "IN"
	NotIn              Comparator = "NOT IN"
	Exists             Comparator = "EXISTS"
	NotExists          Comparator = "NOT EXISTS"
)

type Filter struct {
	Field string
	Cmp   Comparator
	Value any
}
