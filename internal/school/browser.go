package school

import (
	"context"
	"crypto/aes"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	jwLoginURL      = "https://id.ustc.edu.cn/cas/login?service=https%3A%2F%2Fjw.ustc.edu.cn%2Fucas-sso%2Flogin"
	bbLoginURL      = "https://passport.ustc.edu.cn/login?service=https%3A%2F%2Fwww.bb.ustc.edu.cn%2Fwebapps%2Fbb-SSOIntegrationDemo-BBLEARN%2Fexecute%2FauthValidate%2FcustomLogin%3FreturnUrl%3Dhttp%3A%2F%2Fwww.bb.ustc.edu.cn%2Fwebapps%2Fportal%2Fframeset.jsp%26authProviderId%3D_103_1"
	catalogLoginURL = "https://id.ustc.edu.cn/cas/login?service=https%3A%2F%2Fcatalog.ustc.edu.cn%2F"
	authAttempts    = 3
)

const schoolUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"

var errAlreadyAuthenticated = errors.New("already authenticated")

type loginTarget struct {
	loginURL      string
	expectedHost  string
	postLoginURLs []string
}

type casLoginPage struct {
	URL       string
	CryptoKey string
	Execution string
}

func newJWClient(ctx context.Context, creds Credentials) (*http.Client, error) {
	return newAuthenticatedClient(ctx, creds, loginTarget{
		loginURL:      jwLoginURL,
		expectedHost:  "jw.ustc.edu.cn",
		postLoginURLs: []string{"https://jw.ustc.edu.cn/home", "https://jw.ustc.edu.cn/for-std/course-table"},
	})
}

func newBlackboardClient(ctx context.Context, creds Credentials) (*http.Client, error) {
	return newAuthenticatedClientForTargets(ctx, creds, []loginTarget{
		{
			loginURL:      bbNginxAuthLoginURL(bbCalendarPageURL),
			expectedHost:  "www.bb.ustc.edu.cn",
			postLoginURLs: []string{bbCalendarPageURL},
		},
		{
			loginURL:      bbLoginURL,
			expectedHost:  "www.bb.ustc.edu.cn",
			postLoginURLs: []string{bbCalendarPageURL, bbCalendarListURL},
		},
	})
}

func bbNginxAuthLoginURL(rawURL string) string {
	next := hex.EncodeToString([]byte(rawURL))
	service := "http://www.bb.ustc.edu.cn/nginx_auth/login.php?next=" + next
	return "https://passport.ustc.edu.cn/login?service=" + url.QueryEscape(service)
}

func newCatalogClient(ctx context.Context, creds Credentials) (*http.Client, error) {
	return newAuthenticatedClient(ctx, creds, loginTarget{
		loginURL:      catalogLoginURL,
		expectedHost:  "catalog.ustc.edu.cn",
		postLoginURLs: []string{"https://catalog.ustc.edu.cn/", "https://catalog.ustc.edu.cn/api/restricted"},
	})
}

func newAuthenticatedClient(ctx context.Context, creds Credentials, target loginTarget) (*http.Client, error) {
	return newAuthenticatedClientForTargets(ctx, creds, []loginTarget{target})
}

func newAuthenticatedClientForTargets(ctx context.Context, creds Credentials, targets []loginTarget) (*http.Client, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 1; attempt <= authAttempts; attempt++ {
		client, err := openAuthenticatedClientForTargets(ctx, creds, targets)
		if err == nil {
			return client, nil
		}
		lastErr = err
		if attempt < authAttempts {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("failed to authenticate")
	}
	return nil, lastErr
}

func openAuthenticatedClient(ctx context.Context, creds Credentials, target loginTarget) (*http.Client, error) {
	return openAuthenticatedClientForTargets(ctx, creds, []loginTarget{target})
}

func openAuthenticatedClientForTargets(ctx context.Context, creds Credentials, targets []loginTarget) (*http.Client, error) {
	client, err := newSchoolHTTPClient()
	if err != nil {
		return nil, err
	}
	for _, target := range targets {
		if err := authenticateClient(ctx, client, creds, target); err != nil {
			return nil, err
		}
	}
	return client, nil
}

func authenticateClient(ctx context.Context, client *http.Client, creds Credentials, target loginTarget) error {
	loginPage, err := fetchCASLoginPage(ctx, client, target.loginURL)
	if err != nil {
		if errors.Is(err, errAlreadyAuthenticated) {
			return primeAuthenticatedSession(ctx, client, target.postLoginURLs)
		}
		return err
	}

	form, err := buildCASLoginForm(loginPage, creds)
	if err != nil {
		return err
	}

	finalURL, body, err := submitCASLogin(ctx, client, loginPage, form)
	if err != nil {
		return err
	}
	if err := ensureLoginSucceeded(finalURL, body, target.expectedHost); err != nil {
		return err
	}
	if err := primeAuthenticatedSession(ctx, client, target.postLoginURLs); err != nil {
		return err
	}
	return nil
}

func newSchoolHTTPClient() (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}
	return &http.Client{
		Jar:     jar,
		Timeout: 60 * time.Second,
	}, nil
}

func fetchCASLoginPage(ctx context.Context, client *http.Client, loginURL string) (casLoginPage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, loginURL, nil)
	if err != nil {
		return casLoginPage{}, err
	}
	req.Header.Set("User-Agent", schoolUserAgent)

	res, err := client.Do(req)
	if err != nil {
		return casLoginPage{}, fmt.Errorf("open login page: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return casLoginPage{}, responseError(res)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return casLoginPage{}, fmt.Errorf("read login page: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return casLoginPage{}, fmt.Errorf("parse login page: %w", err)
	}

	page := casLoginPage{
		URL:       res.Request.URL.String(),
		CryptoKey: strings.TrimSpace(doc.Find("#login-croypto").First().Text()),
		Execution: strings.TrimSpace(doc.Find("#login-page-flowkey").First().Text()),
	}
	if page.CryptoKey == "" || page.Execution == "" {
		if isNonLoginHost(res.Request.URL.Host) {
			return page, errAlreadyAuthenticated
		}
		return casLoginPage{}, fmt.Errorf("parse login page %s: missing auth fields", res.Request.URL)
	}
	return page, nil
}

func isNonLoginHost(host string) bool {
	return !strings.EqualFold(host, "id.ustc.edu.cn") && !strings.EqualFold(host, "passport.ustc.edu.cn")
}

func buildCASLoginForm(page casLoginPage, creds Credentials) (url.Values, error) {
	encryptedPassword, err := encryptCASPassword(creds.Password, page.CryptoKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt CAS password: %w", err)
	}

	form := url.Values{}
	form.Set("username", creds.Username)
	form.Set("password", encryptedPassword)
	form.Set("type", "UsernamePassword")
	form.Set("_eventId", "submit")
	form.Set("geolocation", "")
	form.Set("execution", page.Execution)
	form.Set("croypto", page.CryptoKey)
	return form, nil
}

func submitCASLogin(ctx context.Context, client *http.Client, page casLoginPage, form url.Values) (*url.URL, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, page.URL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, nil, err
	}
	loginURL, err := url.Parse(page.URL)
	if err != nil {
		return nil, nil, fmt.Errorf("parse login url: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", loginURL.Scheme+"://"+loginURL.Host)
	req.Header.Set("Referer", page.URL)
	req.Header.Set("User-Agent", schoolUserAgent)

	res, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("submit login form: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read login response: %w", err)
	}
	if res.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("submit login form returned %s", res.Status)
	}
	return res.Request.URL, body, nil
}

func ensureLoginSucceeded(finalURL *url.URL, body []byte, expectedHost string) error {
	if finalURL != nil && strings.EqualFold(finalURL.Host, expectedHost) {
		return nil
	}

	html := string(body)
	if strings.Contains(strings.ToLower(html), "dynamic password") || strings.Contains(strings.ToLower(html), ">otp<") {
		return fmt.Errorf("school login requires an OTP challenge that the HTTP client could not complete")
	}

	if msg := extractCASLoginError(html); msg != "" {
		return fmt.Errorf("school login failed: %s", msg)
	}

	if finalURL == nil {
		return fmt.Errorf("school login did not complete")
	}
	return fmt.Errorf("school login did not reach %s; ended at %s", expectedHost, finalURL.Host)
}

func extractCASLoginError(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}

	for _, selector := range []string{
		".ant-form-item-explain-error",
		".alert-error",
		".alert-danger",
		".login-error",
		".error",
		".message",
	} {
		if text := strings.TrimSpace(doc.Find(selector).First().Text()); text != "" {
			return compactWhitespace(text)
		}
	}
	return ""
}

func primeAuthenticatedSession(ctx context.Context, client *http.Client, urls []string) error {
	for _, rawURL := range urls {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", schoolUserAgent)

		res, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("prime authenticated session with %s: %w", rawURL, err)
		}
		if res.StatusCode >= 300 {
			err = responseError(res)
			res.Body.Close()
			return err
		}
		if _, readErr := io.Copy(io.Discard, res.Body); readErr != nil {
			res.Body.Close()
			return fmt.Errorf("read %s: %w", rawURL, readErr)
		}
		res.Body.Close()
	}
	return nil
}

func encryptCASPassword(password, keyBase64 string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(keyBase64))
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	plain := pkcs7Pad([]byte(password), block.BlockSize())
	encrypted := make([]byte, len(plain))
	for offset := 0; offset < len(plain); offset += block.BlockSize() {
		block.Encrypt(encrypted[offset:offset+block.BlockSize()], plain[offset:offset+block.BlockSize()])
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	out := make([]byte, len(data)+padding)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(padding)
	}
	return out
}
