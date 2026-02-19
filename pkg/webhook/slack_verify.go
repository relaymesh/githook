package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

const slackTimestampTolerance = 5 * time.Minute

func slackSignature(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v0:"))
	mac.Write([]byte(timestamp))
	mac.Write([]byte(":"))
	mac.Write(body)
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}

func slackSignatureMatches(secret, timestamp, signature string, body []byte) bool {
	expected := slackSignature(secret, timestamp, body)
	return hmac.Equal([]byte(expected), []byte(signature))
}

func validateSlackTimestamp(timestamp string, now time.Time) error {
	timestamp = strings.TrimSpace(timestamp)
	if timestamp == "" {
		return errors.New("missing slack request timestamp")
	}
	value, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return errors.New("invalid slack request timestamp")
	}
	when := time.Unix(value, 0)
	if when.IsZero() {
		return errors.New("invalid slack request timestamp")
	}
	if now.Sub(when) > slackTimestampTolerance || when.Sub(now) > slackTimestampTolerance {
		return errors.New("stale slack request timestamp")
	}
	return nil
}
