package security

import (
	"errors"
	"testing"
	"time"
)

func TestPlayToken(t *testing.T) {
	now := time.Unix(1000000, 0)
	secret := "test-secret"
	source := "source-1"
	expiry := now.Add(10 * time.Minute).Unix()
	token := Sign(secret, source, expiry)
	cases := []struct {
		name, id, expires, token string
		want                     error
	}{{"valid", source, "1000600", token, nil}, {"expired", source, "999999", token, ErrExpired}, {"invalid", source, "1000600", "bad", ErrInvalid}, {"source tamper", "source-2", "1000600", token, ErrInvalid}, {"expiry tamper", source, "1000599", token, ErrInvalid}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := Verify(secret, tc.id, tc.expires, tc.token, now)
			if !errors.Is(e, tc.want) {
				t.Fatalf("got %v want %v", e, tc.want)
			}
		})
	}
}
