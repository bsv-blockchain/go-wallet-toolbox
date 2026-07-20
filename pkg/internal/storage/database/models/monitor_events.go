package models

type MonitorEvent struct {
	Timestamps
	ID      uint   `gorm:"primarykey;column:id"`
	Event   string `gorm:"type:varchar(64);not null;column:event"`
	Details string `gorm:"type:text;column:details"`
}

func (MonitorEvent) TableName() string {
	return "bsv_monitor_events"
}
