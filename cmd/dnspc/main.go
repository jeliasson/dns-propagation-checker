// Command dnspc checks DNS propagation across many name servers, either from
// the terminal (`dnspc check`) or as a small web app (`dnspc serve`).
package main

import "github.com/jeliasson/dns-propagation-checker/internal/cli"

func main() {
	cli.Main()
}
