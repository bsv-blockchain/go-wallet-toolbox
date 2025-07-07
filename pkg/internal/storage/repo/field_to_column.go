package repo

import (
	"fmt"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"reflect"
)

type fieldToColumn struct {
	lookup      map[string]string
	columnNames map[string]struct{}
}

func newFieldToColumn(fieldNamesStruct, columnNamesStruct any) *fieldToColumn {
	if fieldNamesStruct == nil || columnNamesStruct == nil {
		return nil
	}

	fieldNames := make(map[string]struct{})
	columnNames := make(map[string]struct{})

	fieldNamesType := reflect.TypeOf(fieldNamesStruct)
	fieldNamesValueType := reflect.ValueOf(fieldNamesStruct)

	columnNamesType := reflect.TypeOf(columnNamesStruct)
	columnNamesValueType := reflect.ValueOf(columnNamesStruct)

	if fieldNamesType.Kind() != reflect.Struct || columnNamesType.Kind() != reflect.Struct {
		panic("fieldToColumn: field names and column names must be structs")
	}

	for i := 0; i < fieldNamesType.NumField(); i++ {
		field := fieldNamesType.Field(i)

		if field.Type.Kind() != reflect.String {
			panic("fieldToColumn: field names must be of type string")
		}

		fieldName := fieldNamesValueType.FieldByName(field.Name).String()
		fieldNames[fieldName] = struct{}{}
	}

	for i := 0; i < columnNamesType.NumField(); i++ {
		field := columnNamesType.Field(i)

		if field.Type.Kind() != reflect.String {
			panic("fieldToColumn: column names must be of type string")
		}

		columnName := columnNamesValueType.FieldByName(field.Name).String()
		columnNames[columnName] = struct{}{}
	}

	// direct mapping of field names to column names (the same names)
	lookup := make(map[string]string)
	for columnName := range columnNames {
		if _, ok := fieldNames[columnName]; ok {
			lookup[columnName] = columnName
		}
	}

	return &fieldToColumn{
		lookup:      lookup,
		columnNames: columnNames,
	}
}

func (f *fieldToColumn) Validate() error {
	if len(f.lookup) == 0 || len(f.lookup) != len(f.columnNames) {
		return fmt.Errorf("field to column mapping is empty or does not match the number of column names")
	}

	for fieldName := range f.lookup {
		if _, ok := f.columnNames[fieldName]; !ok {
			return fmt.Errorf("field name %s is not a valid column name", fieldName)
		}
	}

	return nil
}

func (f *fieldToColumn) Mapping(fieldName, columnName string) error {
	if _, ok := f.lookup[fieldName]; ok {
		return fmt.Errorf("field name %s is already mapped to column %s", fieldName, f.lookup[fieldName])
	}

	if _, ok := f.columnNames[columnName]; !ok {
		return fmt.Errorf("column name %s is not a valid column name", columnName)
	}

	f.lookup[fieldName] = columnName
	return nil
}

func (f *fieldToColumn) QueryOptsModifier() func(options *queryopts.Options) {
	if err := f.Validate(); err != nil {
		panic(fmt.Sprintf("fieldToColumn: validation failed: %v", err))
	}
	return func(options *queryopts.Options) {
		if options == nil {
			return
		}

		for i := range options.Filters {
			if options.Filters[i].Field == "" {
				continue
			}

			if columnName, ok := f.lookup[options.Filters[i].Field]; ok {
				options.Filters[i].Field = columnName
			} else {
				continue
			}
		}
	}
}
