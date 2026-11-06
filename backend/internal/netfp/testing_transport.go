package netfp

import "net/http"

// SwapSharedTransportForTesting routes public requests to a deterministic test
// fixture even after another test initialized the shared proxy client. Tests
// must restore it and must not run concurrently with other transport overrides.
func SwapSharedTransportForTesting(transport http.RoundTripper) (restore func()) {
	sharedMu.Lock()
	previous := sharedClient
	sharedClient = &http.Client{Transport: transport}
	sharedMu.Unlock()
	return func() {
		sharedMu.Lock()
		sharedClient = previous
		sharedMu.Unlock()
	}
}
