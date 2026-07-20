package models

import (
	"testing"
	"gorm.io/gorm"
	"gorm.io/driver/sqlite"
)

func TestScratchBasket(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil { panic(err) }
	
	db.AutoMigrate(&Transaction{}, &Output{}, &OutputBasket{})
	
	basket := &OutputBasket{ BasketID: 1, Name: "test" }
	db.Create(basket)
	
	defaultBasketID := uint(1)
	output := &Output{
		BasketID: &defaultBasketID,
		Satoshis: 100,
	}
	
	db.Debug().Create(output)
	
	var out Output
	db.First(&out)
	if out.BasketID == nil {
		t.Fatalf("BasketID is nil!")
	} else {
		t.Logf("Inserted Output BasketID: %v", *out.BasketID)
	}
}
