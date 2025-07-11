package show

import (
	"fmt"
	"strings"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/utils"
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
func WalletSuccess(methodName string, args interface{}, result interface{}) {
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

func HeightOutput(height int64) {
	fmt.Printf("\n%sGet Height: %d%s\n", ColorGreen, height, ColorReset)
}

func IsValidRootForHeightOutput(height uint32, rootHex string, valid bool) {
	fmt.Printf("\n%sHeight: %d | Merkle Root: %s | Valid: %t%s\n", ColorCyan, height, rootHex, valid, ColorReset)
}

func MerklePathOutput(result *wdk.MerklePathResult) {
	utils.PrintMerklePathInfo(result)
    fmt.Println()
    utils.PrintMerklePath(result.MerklePath)
}