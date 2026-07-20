// Command dongle-deploy is a sample plugin. It registers `dongle deploy` and shows
// how a plugin consumes the injected context and brokered credentials.
package main

import (
	"fmt"
	"os"

	sdk "github.com/andreimladin/dongle/pkg/pluginsdk"
)

func main() {
	ctx := sdk.LoadContext()

	sdk.Main(map[string]func([]string) int{
		"": func(_ []string) int {
			fmt.Println("dongle-deploy — try `dongle deploy status` or `dongle deploy run <service>`")
			fmt.Printf("host=%s protocol=%s output=%s\n", ctx.HostVersion, ctx.Protocol, ctx.Output)
			return 0
		},
		"status": func(_ []string) int {
			fmt.Println("all services healthy")
			return 0
		},
		"run": func(args []string) int {
			svc := "default"
			if len(args) > 0 {
				svc = args[0]
			}

			// platform-oidc is declared injectAs:"file" → read via the SDK.
			tok, ok := sdk.Token("platform-oidc")
			if !ok {
				fmt.Fprintln(os.Stderr, "error: no platform-oidc credential was brokered")
				return 1
			}
			// registry-token is declared injectAs:"env" with a custom envVar →
			// read that env var directly.
			reg := os.Getenv("DONGLE_REGISTRY_TOKEN")

			fmt.Printf("deploying %q (invocation %s)\n", svc, ctx.InvocationID)
			fmt.Printf("  platform-oidc token (file): %s\n", redact(tok))
			fmt.Printf("  registry token (env):       %s\n", redact(reg))
			return 0
		},
	})
}

func redact(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:8] + "..."
}
