package cli

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// runHealthcheck probes a running server's /healthz. It exists so the scratch
// based image can declare a HEALTHCHECK: there is no shell or curl in there,
// so the binary has to be able to probe itself.
func runHealthcheck(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("healthcheck", flag.ContinueOnError)
	fs.SetOutput(stderr)

	url := fs.String("url", "http://"+healthHost()+"/healthz", "health endpoint to probe")
	timeout := fs.Duration("timeout", 2*time.Second, "probe timeout")

	fs.Usage = func() {
		fmt.Fprint(stderr, "Usage: dnspc healthcheck [flags]\n\nFlags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	client := &http.Client{Timeout: *timeout}
	// Probing from inside the container; never follow a redirect off-host.
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := client.Get(*url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned %s", *url, resp.Status)
	}
	return nil
}

// healthHost turns the listen address into something dialable, since
// DNSPC_ADDR is usually a bare ":3000".
func healthHost() string {
	addr := envOr("DNSPC_ADDR", ":3000")
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	return addr
}
