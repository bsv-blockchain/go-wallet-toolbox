package testabilities

type TxStatusExpectation struct {
	TxID                string
	ExpectBlockHash     string
	ExpectBlockHeight   int64
	ExpectConfirmations int
}
