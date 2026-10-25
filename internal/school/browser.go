package school

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/Life-USTC/client-core/ustc"
)

const authAttempts = 3

const schoolUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"

type loginTarget struct {
	loginURL      string
	expectedHost  string
	postLoginURLs []string
}

func newJWClient(ctx context.Context, creds Credentials) (*http.Client, error) {
	return newAuthenticatedClient(ctx, creds, loginTarget{
		loginURL:      ustc.CASJWLoginURL,
		expectedHost:  "jw.ustc.edu.cn",
		postLoginURLs: []string{"https://jw.ustc.edu.cn/home", "https://jw.ustc.edu.cn/for-std/course-table"},
	})
}

func newBlackboardClient(ctx context.Context, creds Credentials) (*http.Client, error) {
	return newAuthenticatedClientForTargets(ctx, creds, []loginTarget{
		{
			loginURL:      ustc.BlackboardNginxAuthLoginURL(ustc.BlackboardCalendarPageURL),
			expectedHost:  "www.bb.ustc.edu.cn",
			postLoginURLs: []string{ustc.BlackboardCalendarPageURL},
		},
		{
			loginURL:      ustc.CASBlackboardLoginURL,
			expectedHost:  "www.bb.ustc.edu.cn",
			postLoginURLs: []string{ustc.BlackboardCalendarPageURL, ustc.BlackboardCalendarsURL},
		},
	})
}

func newCatalogClient(ctx context.Context, creds Credentials) (*http.Client, error) {
	return newAuthenticatedClient(ctx, creds, loginTarget{
		loginURL:      ustc.CASCatalogLoginURL,
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
		if errors.Is(err, ustc.ErrCASAlreadyAuthenticated) {
			return primeAuthenticatedSession(ctx, client, target.postLoginURLs)
		}
		return err
	}

	form, err := ustc.CASLoginForm(loginPage, creds.Username, creds.Password)
	if err != nil {
		return err
	}

	finalURL, body, err := submitCASLogin(ctx, client, loginPage, form)
	if err != nil {
		return err
	}
	if err := ustc.CheckCASLoginResult(finalURL, body, target.expectedHost); err != nil {
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

func fetchCASLoginPage(ctx context.Context, client *http.Client, loginURL string) (ustc.CASLoginPage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, loginURL, nil)
	if err != nil {
		return ustc.CASLoginPage{}, err
	}
	req.Header.Set("User-Agent", schoolUserAgent)

	res, err := client.Do(req)
	if err != nil {
		return ustc.CASLoginPage{}, fmt.Errorf("open login page: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return ustc.CASLoginPage{}, responseError(res)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return ustc.CASLoginPage{}, fmt.Errorf("read login page: %w", err)
	}

	return ustc.ParseCASLoginPage(body, res.Request.URL)
}

func submitCASLogin(ctx context.Context, client *http.Client, page ustc.CASLoginPage, form url.Values) (*url.URL, []byte, error) {
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
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read login response: %w", err)
	}
	if res.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("submit login form returned %s", res.Status)
	}
	return res.Request.URL, body, nil
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
			_ = res.Body.Close()
			return err
		}
		if _, readErr := io.Copy(io.Discard, res.Body); readErr != nil {
			_ = res.Body.Close()
			return fmt.Errorf("read %s: %w", rawURL, readErr)
		}
		_ = res.Body.Close()
	}
	return nil
}
