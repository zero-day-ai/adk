package enroll

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
)

// oauth2WithClient injects a custom *http.Client into the oauth2
// package's context so token-exchange calls go through the supplied
// client. Used by tests to point at httptest.Server with self-signed
// TLS.
func oauth2WithClient(ctx context.Context, client *http.Client) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, client)
}
