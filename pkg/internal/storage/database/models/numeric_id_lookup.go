package models

type NumericIDLookup struct {
	NumID     int64  `gorm:"primaryKey;autoIncrement;not null"`
	TableName string `gorm:"not null;uniqueIndex:idx_numeric_id_lookup_table_name_string_id"`
	StringID  string `gorm:"not null;uniqueIndex:idx_numeric_id_lookup_table_name_string_id"`
}
