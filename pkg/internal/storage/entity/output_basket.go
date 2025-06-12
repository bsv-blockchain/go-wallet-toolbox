package entity

type OutputBasket struct {
	Name                    string
	NumberOfDesiredUTXOs    int64
	MinimumDesiredUTXOValue uint64
	UserID                  int
}
