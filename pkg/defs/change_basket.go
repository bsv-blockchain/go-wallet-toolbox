package defs

// ChangeBasket defines configuration for the default change output basket.
type ChangeBasket struct {
	NumberOfDesiredUTXOs    int64  `mapstructure:"number_of_desired_utxos"`
	MinimumDesiredUTXOValue uint64 `mapstructure:"minimum_desired_utxo_value"`
	MaxChangeOutputsPerTx   uint64 `mapstructure:"max_change_outputs_per_tx"`
}

// DefaultChangeBasket returns the default change basket configuration.
func DefaultChangeBasket() ChangeBasket {
	return ChangeBasket{
		NumberOfDesiredUTXOs:    32,
		MinimumDesiredUTXOValue: 1000,
		MaxChangeOutputsPerTx:   8,
	}
}
