module github.com/acme/dongle-deploy

go 1.21

require (
	github.com/spf13/cobra v1.8.0
	github.com/andreimladin/dongle v0.0.0
)

// The SDK isn't published yet; point at the local checkout. Remove once the
// dongle module is pushed and tagged.
replace github.com/andreimladin/dongle => ../..
