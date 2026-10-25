package school

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const (
	casTestPageURL       = "https://id.ustc.edu.cn/cas/login"
	casTestJWLogin       = "https://id.ustc.edu.cn/cas/login?service=https%3A%2F%2Fjw.ustc.edu.cn%2Fucas-sso%2Flogin"
	casTestBBLogin       = "https://passport.ustc.edu.cn/login?service=https%3A%2F%2Fwww.bb.ustc.edu.cn%2Fwebapps%2Fbb-SSOIntegrationDemo-BBLEARN%2Fexecute%2FauthValidate%2FcustomLogin%3FreturnUrl%3Dhttp%3A%2F%2Fwww.bb.ustc.edu.cn%2Fwebapps%2Fportal%2Fframeset.jsp%26authProviderId%3D_103_1"
	casTestCatalogLogin  = "https://id.ustc.edu.cn/cas/login?service=https%3A%2F%2Fcatalog.ustc.edu.cn%2F"
	casTestNginxLogin    = "https://passport.ustc.edu.cn/login?service=http%3A%2F%2Fwww.bb.ustc.edu.cn%2Fnginx_auth%2Flogin.php%3Fnext%3D68747470733a2f2f7777772e62622e757374632e6564752e636e2f776562617070732f63616c656e6461722f76696577506572736f6e616c"
	casTestGraduateLogin = "https://id.ustc.edu.cn/cas/login?service=https%3A%2F%2Fyjs1.ustc.edu.cn%2Fgsapp%2Fsys%2Fkbcxappustc%2F%2Adefault%2Findex.do%3FTHEME%3Dblue%26EMAP_LANG%3Dzh%26min%3D1%23%2Fxskbcx"
	casTestCalendar      = "https://www.bb.ustc.edu.cn/webapps/calendar/viewPersonal"
	casTestLoginHTML     = `<div id="login-croypto">MDEyMzQ1Njc4OWFiY2RlZg==</div><div id="login-page-flowkey">flow-1</div>`
	casTestASCIIForm     = "_eventId=submit&croypto=MDEyMzQ1Njc4OWFiY2RlZg%3D%3D&execution=flow-1&geolocation=&password=c7K9hoi6vxbILr1diHv57A%3D%3D&type=UsernamePassword&username=test-user"
)

type casRequestFixture struct {
	method, url, body, wantBody, location string
	headers                               map[string]string
	cookies                               []*http.Cookie
	status                                int
	networkError, readError               error
}

func casFixtureTransport(t *testing.T, fixtures []casRequestFixture) http.RoundTripper {
	t.Helper()
	calls := 0
	t.Cleanup(func() {
		if calls != len(fixtures) {
			t.Errorf("CAS requests = %d, want %d", calls, len(fixtures))
		}
	})
	return courseTableTransport(func(r *http.Request) (*http.Response, error) {
		if calls >= len(fixtures) {
			t.Fatalf("unexpected CAS request: %s %s", r.Method, r.URL)
		}
		fixture := fixtures[calls]
		calls++
		method := fixture.method
		if method == "" {
			method = http.MethodGet
		}
		if r.Method != method || r.URL.String() != fixture.url || r.Header.Get("User-Agent") != schoolUserAgent {
			t.Fatalf("CAS request %d = %s %s headers=%v, want %s %s", calls, r.Method, r.URL, r.Header, method, fixture.url)
		}
		for name, want := range fixture.headers {
			if got := r.Header.Get(name); got != want {
				t.Fatalf("CAS request %d header %s = %q, want %q", calls, name, got, want)
			}
		}
		if method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil || string(body) != fixture.wantBody {
				t.Fatalf("CAS form = %q, %v; want %q", body, err, fixture.wantBody)
			}
		}
		if fixture.networkError != nil {
			return nil, fixture.networkError
		}
		status := fixture.status
		if status == 0 {
			status = http.StatusOK
		}
		response := importParserResponse(r, status, fixture.body)
		if fixture.location != "" {
			response.Header.Set("Location", fixture.location)
		}
		for _, cookie := range fixture.cookies {
			response.Header.Add("Set-Cookie", cookie.String())
		}
		if fixture.readError != nil {
			response.Body = io.NopCloser(importParserReadError{err: fixture.readError})
		}
		return response, nil
	})
}

func casAlreadyAuthenticatedRequests(loginURL, landing string, primes ...string) []casRequestFixture {
	fixtures := []casRequestFixture{
		{url: loginURL, status: http.StatusFound, location: landing},
		{url: landing, body: `<div id="login-page-flowkey">retained</div>`},
	}
	for _, target := range primes {
		fixtures = append(fixtures, casRequestFixture{url: target})
	}
	return fixtures
}

func TestCASClientTargetsAndAlreadyAuthenticatedPriming(t *testing.T) {
	for _, tc := range []struct {
		name     string
		open     func(context.Context, Credentials) (*http.Client, error)
		requests []casRequestFixture
	}{
		{"jw", newJWClient, casAlreadyAuthenticatedRequests(casTestJWLogin, "https://jw.ustc.edu.cn/home", "https://jw.ustc.edu.cn/home", "https://jw.ustc.edu.cn/for-std/course-table")},
		{"catalog", newCatalogClient, casAlreadyAuthenticatedRequests(casTestCatalogLogin, "https://catalog.ustc.edu.cn/", "https://catalog.ustc.edu.cn/", "https://catalog.ustc.edu.cn/api/restricted")},
		{"graduate", func(ctx context.Context, creds Credentials) (*http.Client, error) {
			return newGraduateYJS1Client(ctx, creds, graduateTestScheduleApp)
		}, casAlreadyAuthenticatedRequests(casTestGraduateLogin, graduateTestScheduleApp, graduateTestScheduleApp)},
		{"blackboard both targets", newBlackboardClient, append(
			casAlreadyAuthenticatedRequests(casTestNginxLogin, casTestCalendar, casTestCalendar),
			casAlreadyAuthenticatedRequests(casTestBBLogin, casTestCalendar, casTestCalendar, "https://www.bb.ustc.edu.cn/webapps/calendar/calendarData/calendars?mode=personal&course_id=")...,
		)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := http.DefaultTransport
			http.DefaultTransport = casFixtureTransport(t, tc.requests)
			t.Cleanup(func() { http.DefaultTransport = original })
			client, err := tc.open(context.Background(), Credentials{Username: "test-user", Password: "test-password"})
			if err != nil || client == nil {
				t.Fatalf("authenticated client = %v, %v", client, err)
			}
			if client.Jar == nil || client.Timeout != 60*time.Second {
				t.Fatalf("native client policy changed: jar=%v timeout=%s", client.Jar, client.Timeout)
			}
		})
	}
}

func TestCASAuthenticationPreservesFormRedirectCookiesAndPriming(t *testing.T) {
	const pageURL = "https://id.ustc.edu.cn/cas/login?flow=selected"
	const homeURL = "https://jw.ustc.edu.cn/home"
	const courseURL = "https://jw.ustc.edu.cn/for-std/course-table"
	client, err := newSchoolHTTPClient()
	if err != nil {
		t.Fatal(err)
	}
	client.Transport = casFixtureTransport(t, []casRequestFixture{
		{url: casTestJWLogin, status: http.StatusFound, location: pageURL, cookies: []*http.Cookie{{Name: "CAS_SEED", Value: "seed", Path: "/cas", Secure: true}}},
		{url: pageURL, headers: map[string]string{"Cookie": "CAS_SEED=seed"}, body: `<span id="login-croypto"> <b>MDEyMzQ1Njc4OWFiY2RlZg==</b> </span><div id="login-croypto">ignored</div><div id="login-page-flowkey"> flow&amp;2
</div>`},
		{method: http.MethodPost, url: pageURL, status: http.StatusFound, location: homeURL,
			headers:  map[string]string{"Cookie": "CAS_SEED=seed", "Content-Type": "application/x-www-form-urlencoded", "Origin": "https://id.ustc.edu.cn", "Referer": pageURL},
			wantBody: "_eventId=submit&croypto=MDEyMzQ1Njc4OWFiY2RlZg%3D%3D&execution=flow%262&geolocation=&password=Dd6NyuDrgc04iNjCb18JK7JRee6pBRX5Fxd%2F2vP2LiHD0gtjF%2FnNaNNGMBDgnAfX&type=UsernamePassword&username=%E7%94%A8%E6%88%B7%2Bone"},
		{url: homeURL, headers: map[string]string{"Cookie": ""}, cookies: []*http.Cookie{{Name: "JW_SESSION", Value: "ready", Path: "/", Secure: true}}, body: `<div class="error">ignored on target host</div>DYNAMIC PASSWORD`},
		{url: homeURL, headers: map[string]string{"Cookie": "JW_SESSION=ready", "Referer": ""}},
		{url: courseURL, headers: map[string]string{"Cookie": "JW_SESSION=ready", "Referer": ""}},
	})
	err = authenticateClient(context.Background(), client, Credentials{Username: "用户+one", Password: "课程密码-and-more-than-one-block"}, loginTarget{
		loginURL: casTestJWLogin, expectedHost: "jw.ustc.edu.cn", postLoginURLs: []string{homeURL, courseURL},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCASAuthenticationProtocolAndHTTPFailuresStopRequests(t *testing.T) {
	networkError := errors.New("disconnected")
	readError := errors.New("short read")
	login := func(post casRequestFixture) []casRequestFixture {
		post.method, post.url, post.wantBody = http.MethodPost, casTestPageURL, casTestASCIIForm
		return []casRequestFixture{{url: casTestPageURL, body: casTestLoginHTML}, post}
	}
	for _, tc := range []struct {
		name     string
		requests []casRequestFixture
		want     string
		cause    error
	}{
		{"open network", []casRequestFixture{{url: casTestPageURL, networkError: networkError}}, fmt.Sprintf("open login page: Get %q: disconnected", casTestPageURL), networkError},
		{"open status", []casRequestFixture{{url: casTestPageURL, status: 502, body: " unavailable "}}, "GET " + casTestPageURL + " returned 502 Bad Gateway: unavailable", nil},
		{"open read", []casRequestFixture{{url: casTestPageURL, readError: readError}}, "read login page: short read", readError},
		{"missing fields", []casRequestFixture{{url: casTestPageURL, body: "Login"}}, "parse login page " + casTestPageURL + ": missing auth fields", nil},
		{"invalid base64 key", []casRequestFixture{{url: casTestPageURL, body: strings.Replace(casTestLoginHTML, "MDEyMzQ1Njc4OWFiY2RlZg==", "!", 1)}}, "encrypt CAS password: illegal base64 data at input byte 0", nil},
		{"invalid AES key", []casRequestFixture{{url: casTestPageURL, body: strings.Replace(casTestLoginHTML, "MDEyMzQ1Njc4OWFiY2RlZg==", "c2hvcnQ=", 1)}}, "encrypt CAS password: crypto/aes: invalid key size 5", nil},
		{"submit network", login(casRequestFixture{networkError: networkError}), fmt.Sprintf("submit login form: Post %q: disconnected", casTestPageURL), networkError},
		{"submit read precedes status", login(casRequestFixture{status: 503, readError: readError}), "read login response: short read", readError},
		{"submit status precedes page error", login(casRequestFixture{status: 503, body: "DYNAMIC PASSWORD"}), "submit login form returned 503 Service Unavailable", nil},
		{"OTP precedes selector error", login(casRequestFixture{body: `<div class="error">failed</div>DYNAMIC PASSWORD`}), "school login requires an OTP challenge that the HTTP client could not complete", nil},
		{"selector priority and whitespace", login(casRequestFixture{body: `<div class="message">later</div><div class="alert-error">middle</div><div class="ant-form-item-explain-error"> Wrong <b>password</b> </div>`}), "school login failed: Wrong password", nil},
		{"unexpected host", login(casRequestFixture{body: "Login"}), "school login did not reach jw.ustc.edu.cn; ended at id.ustc.edu.cn", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: casFixtureTransport(t, tc.requests)}
			err := authenticateClient(context.Background(), client, Credentials{Username: "test-user", Password: "test-password"}, loginTarget{
				loginURL: casTestPageURL, expectedHost: "jw.ustc.edu.cn", postLoginURLs: []string{"https://jw.ustc.edu.cn/home"},
			})
			if err == nil || err.Error() != tc.want {
				t.Fatalf("login error = %v, want %q", err, tc.want)
			}
			if tc.cause != nil && !errors.Is(err, tc.cause) {
				t.Fatalf("login error lost cause: %v", err)
			}
		})
	}
}

func TestCASAuthenticationPrimingStopsAtFirstFailure(t *testing.T) {
	const homeURL = "https://jw.ustc.edu.cn/home"
	const courseURL = "https://jw.ustc.edu.cn/for-std/course-table"
	networkError := errors.New("disconnected")
	readError := errors.New("short read")
	for _, stage := range []int{0, 1} {
		for _, kind := range []string{"network", "status", "read"} {
			t.Run(fmt.Sprintf("prime%d/%s", stage+1, kind), func(t *testing.T) {
				fixtures := casAlreadyAuthenticatedRequests(casTestPageURL, homeURL, homeURL, courseURL)[:3+stage]
				failed := &fixtures[len(fixtures)-1]
				var want string
				switch kind {
				case "network":
					failed.networkError = networkError
					want = fmt.Sprintf("prime authenticated session with %s: Get %q: disconnected", failed.url, failed.url)
				case "status":
					failed.status, failed.body = 503, " unavailable "
					want = "GET " + failed.url + " returned 503 Service Unavailable: unavailable"
				case "read":
					failed.readError = readError
					want = "read " + failed.url + ": short read"
				}
				client := &http.Client{Transport: casFixtureTransport(t, fixtures)}
				err := authenticateClient(context.Background(), client, Credentials{}, loginTarget{
					loginURL: casTestPageURL, expectedHost: "jw.ustc.edu.cn", postLoginURLs: []string{homeURL, courseURL},
				})
				if err == nil || err.Error() != want {
					t.Fatalf("priming error = %v, want %q", err, want)
				}
			})
		}
	}
}

func TestCASClientRetainsRetryAndCancellationPolicy(t *testing.T) {
	original := http.DefaultTransport
	http.DefaultTransport = casFixtureTransport(t, []casRequestFixture{{url: casTestJWLogin}, {url: casTestJWLogin}, {url: casTestJWLogin}})
	t.Cleanup(func() { http.DefaultTransport = original })
	client, err := newJWClient(context.Background(), Credentials{})
	if client != nil || err == nil || err.Error() != "parse login page "+casTestJWLogin+": missing auth fields" {
		t.Fatalf("retry result = %v, %v", client, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client, err = newJWClient(ctx, Credentials{})
	if client != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled login = %v, %v", client, err)
	}
}
