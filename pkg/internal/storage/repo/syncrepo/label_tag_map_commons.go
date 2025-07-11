package syncrepo

import "gorm.io/gorm"

type labelTagMapCommons[Model, ReadModel any] struct {
	db                   *gorm.DB
	tableName            string
	relationUserIDColumn string
	relationNameColumn   string
}

