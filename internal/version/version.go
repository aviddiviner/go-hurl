// Package version holds go-hurl's identity strings.
package version

const (
	// Hurl is the Hurl version targeted by this port.
	Hurl = "8.1.0"
	// GoHurl is the version of go-hurl itself.
	GoHurl = "0.1.0"
	// UserAgent is sent when the user doesn't set a User-Agent.
	UserAgent = "hurl/" + Hurl + " go-hurl/" + GoHurl
)
