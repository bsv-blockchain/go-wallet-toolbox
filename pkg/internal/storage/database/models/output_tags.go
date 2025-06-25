package models

type OutputTags struct {
	OutputID  uint   `gorm:"column:output_id"`
	TagName   string `gorm:"column:tag_name"`
	TagUserID int    `gorm:"column:tag_user_id"`
}
