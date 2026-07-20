package models

// OutputBasket is the database model of the output baskets
type OutputBasket struct {
	Timestamps

	BasketID                uint   `gorm:"column:basketId;primaryKey;autoIncrement"`
	Name                    string `gorm:"column:name;type:varchar(300);uniqueIndex:idx_basket_name_user"`
	UserID                  int    `gorm:"column:userId;uniqueIndex:idx_basket_name_user"`
	NumberOfDesiredUTXOs    int64  `gorm:"column:numberOfDesiredUTXOs;not null;default:6"`
	MinimumDesiredUTXOValue uint64 `gorm:"column:minimumDesiredUTXOValue;not null;default:10000"`
	IsDeleted               bool   `gorm:"column:isDeleted;default:false"`
}
