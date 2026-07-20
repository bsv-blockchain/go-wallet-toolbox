package models

// Setting is the database model of the settings
type Setting struct {
	Timestamps

	StorageIdentityKey string `gorm:"column:storageIdentityKey;primaryKey;type:varchar(130);not null;default:''"`
	StorageName        string `gorm:"column:storageName;type:varchar(128);not null;default:''"`
	Chain              string `gorm:"column:chain;type:varchar(10);not null;default:''"`
	DbType             string `gorm:"column:dbtype;type:varchar(10);not null;default:''"`
	MaxOutputScript    int    `gorm:"column:maxOutputScript;not null;default:0"`
}
