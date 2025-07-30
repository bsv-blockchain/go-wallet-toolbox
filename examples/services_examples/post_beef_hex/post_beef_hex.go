package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

// main demonstrates how to broadcast a single BSV transaction using WalletServices.
//
// In contrast to the post_beef.go example, this example just broadcasts a transaction contained in the provided BEEF hex string.
//
// This example is configurable by setting the following constants at the top of the main function:
//   - `beefHex`: The BEEF where the transaction is contained.
//   - `transactionID`: The transaction ID of the transaction to be broadcasted.
//   - `network`: The BSV network to use (e.g., Testnet or Mainnet). Change to `defs.NetworkMainnet` for mainnet operations.
func main() {
	const (
		// this transaction is already broadcasted, set your own txID and beefHex
		transactionID = "8a6af9c7f4ef19c3056d00efa9866d377df1a6a975e8f4bfced8ac4282f15c8d"
		beefHex       = "0200beef01fde803010100005b83445f8cc99893a5a54f2750874b1ab607c5e6a31e77ba8ce2ade9c6be598703010001000000012e3f4683e173b40a20527fe5719633ba070df649983614886e90e45aecf2ac56000000006a47304402207cfcdbc4c420efa417354861938f6c1a28468b0cdc9dff383691ebf7d8f9279f02201e5378e83bba0f3fdde7d5a73408c3cc6ce9a2a69ea56e67b4834e17b4ca351e4121034d2d6d23fbcb6eefe3e80c47044e36797dcb80d0ac5e96e732ef03c3c550a116ffffffff0141860100000000001976a91494677c56fa2968644c90a517214338b4139899ce88ac000000000001000000015b83445f8cc99893a5a54f2750874b1ab607c5e6a31e77ba8ce2ade9c6be5987000000006b483045022100e719806b81b41b130d95c7efb33b3af2f79db96e0545ee6567623a122a31346f0220326c284d21e8fbc0a7541c5723b37d36462d52d0a0286bdd2c277b3429a7241f4121034d2d6d23fbcb6eefe3e80c47044e36797dcb80d0ac5e96e732ef03c3c550a116ffffffff0240860100000000001976a91418ea4c892504750609961be2a675ae569db7fe9088ac000000000000000011006a0e66617563657420696e64657820300000000000010000000116e0538eb4e0431a869547246ff52bfc1b89e241a77a0817cf04d735e7f90a7e0000000000ffffffff2010a40000000000001976a914dbc0a7c84983c5bf199b7b2d41b3acf0408ee5aa88ac6f0a0000000000001976a9144b18480346a7d91e370ed048317dde841f4b5e3188ac59080000000000001976a9148720eca3cec7738208b3e5035608786666015cf888ac56080000000000001976a914aa9ab076939c35f870af2b477cbcfc00b5d8ec6388ac68070000000000001976a914a6d40cab2537236269564c288f2b6bcfded4518e88ac4e070000000000001976a914163b3b136d71f268c0ef03eea3a46fcbb2339e1988acf3050000000000001976a9147680bc87ca246cf539acd97976f1639ad569731588ac1b070000000000001976a914385d52fa34c1308f6b38407de0f8bb32e1d730c988ac3c060000000000001976a914f3fb84d9017b9301d68b237ea134ec1a919d0c5188acbc040000000000001976a914483b49d621dfe0beed36f9c14f6a5288fba4af8a88acfb070000000000001976a914c51857053b6bebd9a035b329a27055d8d0997e9688ac7c070000000000001976a9145acbbde942a8518ee3b1a4d7728213c38f2a998b88ac15090000000000001976a9148c899a7ffeae3c652802bbef2ca67a813e6c7b9e88acad080000000000001976a9149038040aadbe0c995919700faed68101d49e5f9c88acc4080000000000001976a91402d300e18cfb1a078d48f10f6eaeb9c9d025e75288ac81090000000000001976a91480f671f5703b54e558158f25c8774f1755256e3488ac4b070000000000001976a91421d2afb5cf31eb2f7a574456505db79648eebb2088ac15050000000000001976a9143fa755f8e622314a01ea47779667215b7ee67d8d88acd2050000000000001976a914e097d5075ec68003e862d52d70d132dda8d455f588ace9050000000000001976a9144b04c1740154b6d75e63e98833aa011d6056b24988ac81050000000000001976a914ae5c3b18357fa03054846cd50f7ff7719421f0eb88ac1a070000000000001976a914f79b25213ea079db3cd76d93870509ae466dde9788ac9b060000000000001976a914d0150b442b14100842a29af71d5ea53a5b343a3f88acda090000000000001976a9145fbe0942470ec07eabb8e4a535083a979083ed3788ac5a080000000000001976a914d0ed072ebb2ca49717ec266e0264788c23c0f48b88ac7b070000000000001976a914be2de8fdab2bcbc9c4e7e64cf71ee9b73f24613888aca3080000000000001976a914670ee6440005d295dd6e7baa5047fd310bcf2b3a88ac48070000000000001976a9147e81ffd7a7443472d40fb25354514465ea4bacd088ac2e070000000000001976a91422dc1b875eb407059bf070161424eb087d1a3fdf88ac40060000000000001976a914a8cdc0fca96f84f87a0f92f2a7f4c5569f4ee83188ac3d060000000000001976a914d8b2d28bb736cb900c594539641b45ef21762cf188ac40040000000000001976a914d954d012064c2c9cbef144155442137ec975ac4588ac00000000"
		network       = defs.NetworkTestnet
	)

	// //Set to LevelDebug to see http request logs
	slog.SetLogLoggerLevel(slog.LevelDebug)

	beefBytes, err := hex.DecodeString(beefHex)
	if err != nil {
		panic(fmt.Errorf("could not decode beef hex: %w", err))
	}

	beef, err := transaction.NewBeefFromBytes(beefBytes)
	if err != nil {
		panic(fmt.Errorf("could not create beef from bytes: %w", err))
	}

	serviceCfg := defs.DefaultServicesConfig(network)

	walletServices := services.New(slog.Default(), serviceCfg)

	results, err := walletServices.PostBEEF(context.Background(), beef, []string{transactionID})
	if err != nil {
		panic(err)
	}

	for _, result := range results {
		fmt.Println("===========================================================")
		fmt.Printf("Service %s PostBEEF result:", result.Name)
		if !result.Success() {
			fmt.Println("	Error:", result.Error)
		} else {
			fmt.Println("	Success:")
			for _, resultForTxID := range result.PostedBEEFResult.TxIDResults {
				fmt.Println("		TX ID:", resultForTxID.TxID)
				fmt.Println("		Result:	", resultForTxID.Result)
				if resultForTxID.Result == "error" {
					fmt.Println("		Error:	", resultForTxID.Error)
				} else {
					fmt.Println("		AlreadyKnown:	", resultForTxID.AlreadyKnown)
					fmt.Println("		DoubleSpend:	", resultForTxID.DoubleSpend)
					fmt.Println("		BlockHash:	", resultForTxID.BlockHash)
					fmt.Println("		BlockHeight:	", resultForTxID.BlockHeight)
					fmt.Println("		MerklePath:	", resultForTxID.MerklePath)
					fmt.Println("		CompetingTxs:	", resultForTxID.CompetingTxs)
					fmt.Println("		Notes:	", resultForTxID.Notes)
					fmt.Println("		Data:	", resultForTxID.Data)
				}
			}
		}
	}

}
