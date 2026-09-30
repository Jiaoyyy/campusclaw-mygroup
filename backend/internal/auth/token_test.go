package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestTokenRequiresBearer(t *testing.T) {
	valid := strings.Repeat("a", 64)
	r := httptest.NewRequest("GET", "/api/me", nil)
	r.AddCookie(&http.Cookie{Name: "campusclaw_session", Value: "old-cookie"})
	r.Header.Set("Authorization", "Bearer "+valid)
	if got, ok := requestToken(r); !ok || got != valid {
		t.Fatalf("bearer not selected: %q %v", got, ok)
	}
	r.Header.Set("Authorization", "Bearer broken")
	if got, ok := requestToken(r); ok || got != "" {
		t.Fatal("malformed bearer fell back to cookie")
	}
	r.Header.Del("Authorization")
	if got, ok := requestToken(r); ok || got != "" {
		t.Fatal("cookie-only request was authenticated")
	}
}
