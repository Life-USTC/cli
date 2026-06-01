package school

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"hash"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func CurrentOTP(raw string, now time.Time) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("empty totp value")
	}
	if len(value) == 6 && isDigits(value) {
		return value, nil
	}

	secret, algorithm, digits, period, err := parseOTPValue(value)
	if err != nil {
		return "", err
	}

	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		return "", fmt.Errorf("decode totp secret: %w", err)
	}

	counter := uint64(now.Unix() / int64(period))
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	mac := hmac.New(hashForAlgorithm(algorithm), key)
	_, _ = mac.Write(buf)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	code := (int(sum[offset])&0x7f)<<24 |
		(int(sum[offset+1])&0xff)<<16 |
		(int(sum[offset+2])&0xff)<<8 |
		(int(sum[offset+3]) & 0xff)

	mod := 1
	for range digits {
		mod *= 10
	}
	format := "%0" + strconv.Itoa(digits) + "d"
	return fmt.Sprintf(format, code%mod), nil
}

func parseOTPValue(raw string) (secret, algorithm string, digits, period int, err error) {
	algorithm = "SHA1"
	digits = 6
	period = 30

	if strings.HasPrefix(raw, "otpauth://") {
		parsed, parseErr := url.Parse(raw)
		if parseErr != nil {
			return "", "", 0, 0, fmt.Errorf("parse otpauth url: %w", parseErr)
		}
		query := parsed.Query()
		secret = strings.ToUpper(strings.TrimSpace(query.Get("secret")))
		if value := strings.TrimSpace(query.Get("algorithm")); value != "" {
			algorithm = strings.ToUpper(value)
		}
		if value := strings.TrimSpace(query.Get("digits")); value != "" {
			digits, err = strconv.Atoi(value)
			if err != nil {
				return "", "", 0, 0, fmt.Errorf("parse otp digits: %w", err)
			}
		}
		if value := strings.TrimSpace(query.Get("period")); value != "" {
			period, err = strconv.Atoi(value)
			if err != nil {
				return "", "", 0, 0, fmt.Errorf("parse otp period: %w", err)
			}
		}
	} else {
		secret = strings.ToUpper(strings.TrimSpace(raw))
	}

	secret = strings.NewReplacer(" ", "", "-", "").Replace(secret)
	if secret == "" {
		return "", "", 0, 0, fmt.Errorf("missing totp secret")
	}
	if digits <= 0 {
		return "", "", 0, 0, fmt.Errorf("invalid otp digits: %d", digits)
	}
	if period <= 0 {
		return "", "", 0, 0, fmt.Errorf("invalid otp period: %d", period)
	}

	return secret, algorithm, digits, period, nil
}

func hashForAlgorithm(name string) func() hash.Hash {
	switch strings.ToUpper(name) {
	case "SHA256":
		return sha256.New
	case "SHA512":
		return sha512.New
	default:
		return sha1.New
	}
}

func isDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
