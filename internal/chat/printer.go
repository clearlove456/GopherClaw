package chat

import "fmt"

const (
	cyan   = "\033[36m"
	green  = "\033[32m"
	yellow = "\033[33m"
	dim    = "\033[2m"
	reset  = "\033[0m"
	bold   = "\033[1m"
)

func coloredPrompt() string {
	return fmt.Sprintf("%s%sYou > %s", cyan, bold, reset)
}

func printAssistant(text string) {
	fmt.Printf("\n%s%sAssistant:%s %s\n\n", green, bold, reset, text)
}

func printInfo(text string) {
	fmt.Printf("%s%s%s\n", dim, text, reset)
}

func printAPIError(err error) {
	fmt.Printf("\n%sAPI Error: %v%s\n\n", yellow, err, reset)
}
