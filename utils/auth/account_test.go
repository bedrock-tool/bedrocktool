package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// A refresh token Microsoft refuses for good (#286, #201, #134) is told
// apart from other errors, using the error oauth2 really returns.
func TestLoginRevoked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"invalid_grant","error_description":"The user could not be authenticated or the grant is expired."}`)
	}))
	defer srv.Close()

	conf := &oauth2.Config{ClientID: "test", Endpoint: oauth2.Endpoint{TokenURL: srv.URL}}
	expired := &oauth2.Token{RefreshToken: "old", Expiry: time.Now().Add(-time.Hour)}
	_, err := conf.TokenSource(context.Background(), expired).Token()
	if err == nil {
		t.Fatal("refresh succeeded")
	}
	if !loginRevoked(err) {
		t.Fatalf("loginRevoked(%v) = false", err)
	}
	if !loginRevoked(fmt.Errorf("request Live Connect token: %w", err)) {
		t.Fatal("wrapped invalid_grant not recognised")
	}

	for _, other := range []error{
		errors.New("dial tcp: i/o timeout"),
		&oauth2.RetrieveError{ErrorCode: "temporarily_unavailable"},
		nil,
	} {
		if loginRevoked(other) {
			t.Errorf("loginRevoked(%v) = true", other)
		}
	}
}

func TestForgetOnlyCurrentAccount(t *testing.T) {
	t.Chdir(t.TempDir())
	a := &authSrv{log: Auth.log}
	current := &Account{name: "test"}
	a.account.Store(current)

	a.forget(&Account{name: "other"})
	if !a.LoggedIn() {
		t.Fatal("forgetting another account logged out the current one")
	}
	a.forget(current)
	if a.LoggedIn() {
		t.Fatal("still logged in after forget")
	}
}
