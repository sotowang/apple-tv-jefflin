package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"time"
)

var ErrExpired = errors.New("play token expired")
var ErrInvalid = errors.New("invalid play token")

func Equal(a, b string) bool {
	ha := sha256.Sum256([]byte(a))
	hb := sha256.Sum256([]byte(b))
	return hmac.Equal(ha[:], hb[:])
}
func Sign(secret, sourceID string, expires int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(sourceID + "\n" + strconv.FormatInt(expires, 10)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func Verify(secret, sourceID, expires, token string, now time.Time) error {
	ts, e := strconv.ParseInt(expires, 10, 64)
	if e != nil || ts <= 0 {
		return ErrInvalid
	}
	if now.Unix() > ts {
		return ErrExpired
	}
	if ts > now.Add(10*time.Minute+time.Minute).Unix() {
		return ErrInvalid
	}
	if !Equal(Sign(secret, sourceID, ts), token) {
		return ErrInvalid
	}
	return nil
}
