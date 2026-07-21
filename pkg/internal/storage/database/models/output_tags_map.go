package models

type OutputTagsMap struct {
	Timestamps
	OutputTagID uint `gorm:"column:outputTagId;primaryKey"`
	OutputID    uint `gorm:"column:outputId;primaryKey;index"`
	IsDeleted   bool `gorm:"column:isDeleted;default:false"`
}

func (OutputTagsMap) TableName() string {
	return "bsv_output_tags_map"
}
