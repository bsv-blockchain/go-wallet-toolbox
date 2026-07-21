package models

type OutputTag struct {
	Timestamps
	OutputTagID uint   `gorm:"primarykey;column:outputTagId"`
	Tag         string `gorm:"type:varchar(150);column:tag;uniqueIndex:idx_output_tags_tag_user"`
	UserID      int    `gorm:"column:userId;uniqueIndex:idx_output_tags_tag_user"`
	IsDeleted   bool   `gorm:"column:isDeleted;default:false"`
}

func (OutputTag) TableName() string {
	return "bsv_output_tags"
}
