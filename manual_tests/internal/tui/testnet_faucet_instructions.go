package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Style for the main header
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			PaddingLeft(1).
			PaddingRight(1)

	// Style for the "NOTICE" and "WARNING" labels
	noticeLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("220")) // Yellow

	// Style for the "ADDRESS" label
	addressLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("36")) // Cyan

	// Style for the address itself
	addressValueStyle = lipgloss.NewStyle().
				PaddingLeft(3)

	// Style for the "Available Faucets" heading
	faucetHeaderStyle = lipgloss.NewStyle().
				Bold(true)

	// Style for the list of faucet links
	linkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("36"))
)

func RenderTestnetFaucetInstructions(address string) string {
	var b strings.Builder

	// Header
	b.WriteString(headerStyle.Render("FAUCET ADDRESS"))
	b.WriteString("\n\n")

	// Notice
	fmt.Fprintf(&b, "%s %s\n", noticeLabelStyle.Render("💡 NOTICE:"), "You need to fund this address from a testnet faucet")
	b.WriteString("\n")

	// Address
	b.WriteString(addressLabelStyle.Render("📧 ADDRESS:"))
	b.WriteString("\n")
	b.WriteString(addressValueStyle.Render(address))
	b.WriteString("\n\n")

	// Faucet List
	b.WriteString(faucetHeaderStyle.Render("Available Testnet Faucets:"))
	b.WriteString("\n")
	fmt.Fprintf(&b, "• %s\n", linkStyle.Render("https://scrypt.io/faucet"))
	fmt.Fprintf(&b, "• %s\n", linkStyle.Render("https://witnessonchain.com/faucet/tbsv"))
	b.WriteString("\n")

	// Warning
	fmt.Fprintf(&b, "%s %s\n", noticeLabelStyle.Render("⚠️  WARNING:"), "Make sure to use TESTNET faucets only!")
	b.WriteString("\n")

	return b.String()
}
