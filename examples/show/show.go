package show

import (
	"fmt"
	"strings"
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

// Notice displays an educational notice or important information
func Notice(message string) {
	fmt.Printf("\n%s💡 NOTICE:%s %s\n", ColorYellow+ColorBold, ColorReset, message)
}

// Success displays a success message
func Success(message string) {
	fmt.Printf("%s✅ SUCCESS:%s %s\n", ColorGreen+ColorBold, ColorReset, message)
}

// Error displays an error message
func Error(message string) {
	fmt.Printf("%s❌ ERROR:%s %s\n", ColorRed+ColorBold, ColorReset, message)
}

// Warning displays a warning message
func Warning(message string) {
	fmt.Printf("%s⚠️  WARNING:%s %s\n", ColorYellow+ColorBold, ColorReset, message)
}

// Address displays a blockchain address in a formatted way
func Address(address string) {
	fmt.Printf("\n%s📧 ADDRESS:%s\n", ColorCyan+ColorBold, ColorReset)
	fmt.Printf("   %s\n", address)
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
func Info(label, value string) {
	fmt.Printf("%s%s:%s %s\n", ColorCyan, label, ColorReset, value)
}

// FaucetInstructions displays formatted faucet instructions
func FaucetInstructions(address string) {
	Header("FAUCET ADDRESS")
	Notice("You need to fund this address from a testnet faucet")
	Address(address)
	fmt.Println("")
	fmt.Printf("%sAvailable Testnet Faucets:%s\n", ColorBold, ColorReset)
	fmt.Println("• https://scrypt.io/faucet")
	fmt.Println("• https://witnessonchain.com/faucet/tbsv")
	fmt.Println("")
	Warning("Make sure to use TESTNET faucets only!")
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
