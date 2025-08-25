package scopes

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/go-softwarelab/common/pkg/to"
	"gorm.io/gen"
	"gorm.io/gen/field"
)

func PaginateForGen(getter genTableGetter, page *queryopts.Paging) func(gen.Dao) gen.Dao {
	page.ApplyDefaults()
	return func(dao gen.Dao) gen.Dao {
		sortBy, ok := getter.GetFieldByName(page.SortBy)
		if !ok {
			_ = dao.AddError(fmt.Errorf("field %s not found", page.SortBy))
			return dao
		}

		sortByExpr := to.If(page.IsDesc(), sortBy.Desc).Else(sortBy.Asc)

		return dao.Order(sortByExpr).Offset(page.Offset).Limit(page.Limit)
	}
}

func SinceForGen(getter genTableGetter, since *queryopts.Since) func(gen.Dao) gen.Dao {
	since.ApplyDefaults()
	return func(dao gen.Dao) gen.Dao {
		sinceByExpr, ok := getter.GetFieldByName(since.Field)
		if !ok {
			_ = dao.AddError(fmt.Errorf("field %s not found", since.Field))
			return dao
		}

		sinceByField := field.NewTime(getter.TableName(), sinceByExpr.ColumnName().String())

		return dao.Where(sinceByField.Gte(since.Time))
	}
}

func UntilForGen(getter genTableGetter, until *queryopts.Until) func(gen.Dao) gen.Dao {
	until.ApplyDefaults()
	return func(dao gen.Dao) gen.Dao {
		untilByExpr, ok := getter.GetFieldByName(until.Field)
		if !ok {
			_ = dao.AddError(fmt.Errorf("field %s not found", until.Field))
			return dao
		}

		untilByField := field.NewTime(getter.TableName(), untilByExpr.ColumnName().String())

		return dao.Where(untilByField.Lte(until.Time))
	}
}

type genTableGetter interface {
	GetFieldByName(fieldName string) (field.OrderExpr, bool)
	TableName() string
}

func FromQueryOptsForGen(getter genTableGetter, opts []queryopts.Options) []func(gen.Dao) gen.Dao {
	options := queryopts.MergeOptions(opts)

	var sc []func(gen.Dao) gen.Dao
	if options.Page != nil {
		sc = append(sc, PaginateForGen(getter, options.Page))
	}
	if options.Since != nil {
		sc = append(sc, SinceForGen(getter, options.Since))
	}
	if options.Until != nil {
		sc = append(sc, UntilForGen(getter, options.Until))
	}

	return sc
}
