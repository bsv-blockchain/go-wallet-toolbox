package models

type OutputTagsMap struct {
	Timestamps
	OutputTagID uint `gorm:"column:output_tag_id;primaryKey"`
	OutputID    uint `gorm:"column:output_id;primaryKey"`
	IsDeleted   bool `gorm:"column:isDeleted;default:false"`
}

func (OutputTagsMap) TableName() string {
	return "bsv_output_tags_map"
}
