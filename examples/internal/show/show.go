package show

import (
	"fmt"
	"strings"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
	ColorBold   = "\033[1m"
)

// Step displays a formatted step in the process with an actor and description
func Step(actor, description string) {
	fmt.Printf("\n%s=== STEP ===%s\n", ColorBlue+ColorBold, ColorReset)
	fmt.Printf("%s%s%s is performing: %s\n", ColorGreen, actor, ColorReset, description)
	fmt.Println(strings.Repeat("-", 50))
}

// Success displays a success message
func Success(message string) {
	fmt.Printf("%s✅ SUCCESS:%s %s\n", ColorGreen+ColorBold, ColorReset, message)
}

// Error displays an error message
func Error(message string) {
	fmt.Printf("%s❌ ERROR:%s %s\n", ColorRed+ColorBold, ColorReset, message)
}

// Transaction displays transaction information
func Transaction(txid string) {
	fmt.Printf("\n%s🔗 TRANSACTION:%s\n", ColorPurple+ColorBold, ColorReset)
	fmt.Printf("   TxID: %s\n", txid)
}

// Separator prints a visual separator
func Separator() {
	fmt.Println(strings.Repeat("=", 60))
}

// Header displays a section header
func Header(title string) {
	fmt.Printf("\n%s", strings.Repeat("=", 60))
	fmt.Printf("\n%s%s%s\n", ColorBold, strings.ToUpper(title), ColorReset)
	fmt.Printf("%s\n", strings.Repeat("=", 60))
}

// Info displays general information
func Info(label string, value interface{}) {
	fmt.Printf("%s%s:%s %+v\n", ColorCyan, label, ColorReset, value)
}

// FaucetInstructions displays formatted faucet instructions
func FaucetInstructions(address string) {
	Header("FAUCET ADDRESS")
	fmt.Printf("\n%s💡 NOTICE:%s %s\n", ColorYellow+ColorBold, ColorReset, "You need to fund this address from a testnet faucet")
	fmt.Printf("\n%s📧 ADDRESS:%s\n", ColorCyan+ColorBold, ColorReset)
	fmt.Printf("   %s\n", address)
	fmt.Println("")
	fmt.Printf("%sAvailable Testnet Faucets:%s\n", ColorBold, ColorReset)
	fmt.Println("• https://scrypt.io/faucet")
	fmt.Println("• https://witnessonchain.com/faucet/tbsv")
	fmt.Println("")
	fmt.Printf("%s⚠️  WARNING:%s %s\n", ColorYellow+ColorBold, ColorReset, "Make sure to use TESTNET faucets only!")
}

// ProcessStart indicates the beginning of a process
func ProcessStart(processName string) {
	fmt.Printf("\n%s🚀 STARTING:%s %s\n", ColorGreen+ColorBold, ColorReset, processName)
	Separator()
}

// ProcessComplete indicates the completion of a process
func ProcessComplete(processName string) {
	Separator()
	fmt.Printf("%s🎉 COMPLETED:%s %s\n\n", ColorGreen+ColorBold, ColorReset, processName)
}

// WalletSuccess displays a successful wallet method call with its arguments and result
func WalletSuccess(methodName string, args, result interface{}) {
	fmt.Printf("\n%s WALLET CALL:%s %s%s%s\n", ColorBlue+ColorBold, ColorReset, ColorGreen, methodName, ColorReset)
	fmt.Printf("%sArgs:%s %+v\n", ColorCyan, ColorReset, args)
	fmt.Printf("%s✅ Result:%s %+v\n", ColorGreen+ColorBold, ColorReset, result)
}

// WalletError displays a failed wallet method call with its arguments and error
func WalletError(methodName string, args interface{}, err error) {
	fmt.Printf("\n%s WALLET CALL:%s %s%s%s\n", ColorBlue+ColorBold, ColorReset, ColorRed, methodName, ColorReset)
	fmt.Printf("%sArgs:%s %+v\n", ColorCyan, ColorReset, args)
	fmt.Printf("%s❌ Error:%s %v\n", ColorRed+ColorBold, ColorReset, err)
}

// PrintTable replicates the tiny helper used in the other examples
func PrintTable(title string, headers []string, rows [][]string) {
	if title != "" {
		fmt.Printf("%s\n", title)
	}
	colW := make([]int, len(headers))
	for i, h := range headers {
		colW[i] = len(h)
	}
	for _, r := range rows {
		for i, cell := range r {
			if len(cell) > colW[i] {
				colW[i] = len(cell)
			}
		}
	}
	printRow := func(cells []string) {
		for i, c := range cells {
			fmt.Printf("%-*s  ", colW[i], c)
		}
		fmt.Println()
	}

	printRow(headers)
	for i := range headers {
		fmt.Printf("%s  ", strings.Repeat("-", colW[i]))
	}
	fmt.Println()
	for _, r := range rows {
		printRow(r)
	}
}

func CurrentHeightOutput(height uint32) {
	fmt.Printf("\n%sGet Height: %d%s\n", ColorGreen, height, ColorReset)
}

func IsValidRootForHeightOutput(height uint32, rootHex string, valid bool) {
	fmt.Printf("\n%sHeight: %d | Merkle Root: %s | Valid: %t%s\n", ColorCyan, height, rootHex, valid, ColorReset)
}

func MerklePathOutput(result *wdk.MerklePathResult) {
	printMerklePathInfo(result)
	fmt.Println()
	printMerklePath(result.MerklePath)
}

// ChainTipHeaderOutput prints a chain-tip block header in a table.
func ChainTipHeaderOutput(h *wdk.ChainBlockHeader) {
	if h == nil {
		Error("nil ChainBlockHeader passed to ChainTipHeaderOutput")
		return
	}

	headers := []string{
		"Height", "Hash", "Version",
		"Prev-Hash", "Merkle-Root", "Time", "Bits", "Nonce",
	}
	rows := [][]string{{
		fmt.Sprint(h.Height),
		h.Hash,
		fmt.Sprintf("%08x", h.Version),
		h.PreviousHash,
		h.MerkleRoot,
		fmt.Sprint(h.Time),
		fmt.Sprintf("%08x", h.Bits),
		fmt.Sprint(h.Nonce),
	}}

	PrintTable("Chain Tip Header:", headers, rows)
}

func RawTxOutput(result *wdk.RawTxResult) {
	if result == nil {
		Error("nil RawTxResult passed to RawTxOutput")
		return
	}

	Header("RAW TRANSACTION RESULT")
	fmt.Printf("%sService:%s %s\n", ColorCyan, ColorReset, result.Name)
	fmt.Printf("%sTxID:   %s%s\n", ColorCyan, result.TxID, ColorReset)
	fmt.Printf("%sRawTx:  %x%s\n", ColorCyan, result.RawTx, ColorReset)
}

// PostBEEFOutput displays the results of PostBEEF operations from multiple services
func PostBEEFOutput(results []*wdk.PostFromBEEFServiceResult) {
	if len(results) == 0 {
		Error("No PostBEEF results to display")
		return
	}

	Header("POST BEEF RESULTS")

	for _, result := range results {
		fmt.Printf("\n%s%s========================================%s\n", ColorBlue+ColorBold, ColorReset, ColorReset)
		fmt.Printf("%sService: %s%s%s\n", ColorCyan+ColorBold, ColorGreen, result.Name, ColorReset)

		if !result.Success() {
			fmt.Printf("%s❌ Error:%s %v\n", ColorRed+ColorBold, ColorReset, result.Error)
		} else {
			fmt.Printf("%s✅ Success%s\n", ColorGreen+ColorBold, ColorReset)

			for _, txResult := range result.PostedBEEFResult.TxIDResults {
				fmt.Printf("\n%s  📋 Transaction Result:%s\n", ColorPurple+ColorBold, ColorReset)
				fmt.Printf("    %sTX ID:%s %s\n", ColorCyan, ColorReset, txResult.TxID)
				fmt.Printf("    %sResult:%s %s\n", ColorCyan, ColorReset, txResult.Result)

				if txResult.Result == "error" {
					fmt.Printf("    %sError:%s %v\n", ColorRed, ColorReset, txResult.Error)
				} else {
					fmt.Printf("    %sAlready Known:%s %t\n", ColorCyan, ColorReset, txResult.AlreadyKnown)
					fmt.Printf("    %sDouble Spend:%s %t\n", ColorCyan, ColorReset, txResult.DoubleSpend)
					fmt.Printf("    %sBlock Hash:%s %s\n", ColorCyan, ColorReset, txResult.BlockHash)
					fmt.Printf("    %sBlock Height:%s %d\n", ColorCyan, ColorReset, txResult.BlockHeight)
					fmt.Printf("    %sMerkle Path:%s %v\n", ColorCyan, ColorReset, txResult.MerklePath)
					fmt.Printf("    %sCompeting TXs:%s %v\n", ColorCyan, ColorReset, txResult.CompetingTxs)
					fmt.Printf("    %sNotes:%s %v\n", ColorCyan, ColorReset, txResult.Notes)
					fmt.Printf("    %sData:%s %s\n", ColorCyan, ColorReset, txResult.Data)
				}
			}
		}
	}
}

// ScriptHashHistoryOutput displays the history of a script hash
func ScriptHashHistoryOutput(result *wdk.ScriptHistoryResult) {
	if result == nil {
		Error("nil ScriptHistoryResult passed to ScriptHashHistoryOutput")
		return
	}

	Header("SCRIPT HASH HISTORY")
	fmt.Printf("%sService:%s %s\n", ColorCyan, ColorReset, result.Name)
	fmt.Printf("%sScriptHash:%s %s\n", ColorCyan, ColorReset, result.ScriptHash)

	headers := []string{"TxHash", "Status", "Block Height"}
	rows := make([][]string, 0, len(result.History))

	for _, item := range result.History {
		status := "Unconfirmed"
		height := "-"
		if item.Height != nil && *item.Height > 0 {
			status = "Confirmed"
			height = fmt.Sprint(*item.Height)
		}
		rows = append(rows, []string{item.TxHash, status, height})
	}

	PrintTable("Transaction History:", headers, rows)
}

func GetUtxoStatusOutput(result *wdk.UtxoStatusResult) {
	if result == nil {
		Error("nil UtxoStatusResult passed to GetUtxoStatusOutput")
		return
	}

	Header("UTXO STATUS RESULT")
	fmt.Printf("%sService:%s %s\n", ColorCyan, ColorReset, result.Name)
	fmt.Printf("%sIs UTXO:%s %t\n", ColorCyan, ColorReset, result.IsUtxo)
	fmt.Printf("%sDetails:%s\n", ColorCyan, ColorReset)

	if len(result.Details) == 0 {
		fmt.Println("  No UTXOs found.")
		return
	}

	for _, detail := range result.Details {
		fmt.Printf("  TxID: %s | Index: %d | Height: %d | Satoshis: %d\n",
			detail.TxID, detail.Index, detail.Height, detail.Satoshis)
	}
}

// GetStatusForTxIDsOutput pretty-prints the GetStatusForTxIDs result.
func GetStatusForTxIDsOutput(res *wdk.GetStatusForTxIDsResult) {
	if res == nil {
		Error("nil GetStatusForTxIDsResult passed")
		return
	}

	Header("TX STATUS (MULTI)")
	fmt.Printf("%sService:%s %s\n", ColorCyan, ColorReset, res.Name)
	fmt.Printf("%sOverall:%s %s\n\n", ColorCyan, ColorReset, res.Status)

	headers := []string{"TxID", "Status", "Depth"}
	rows := make([][]string, 0, len(res.Results))

	for _, it := range res.Results {
		depth := "-"
		if it.Depth != nil {
			depth = fmt.Sprint(*it.Depth)
		}
		rows = append(rows, []string{
			it.TxID,
			it.Status,
			depth,
		})
	}
	PrintTable("Per-TX status:", headers, rows)
}

func Beef(beefHex string) {
	Header("BEEF HEX")

	fmt.Printf("%q", beefHex)
}
