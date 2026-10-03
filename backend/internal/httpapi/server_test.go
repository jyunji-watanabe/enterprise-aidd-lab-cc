package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/httpapi"
	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/seed"
	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/service"
	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/store"
	"golang.org/x/crypto/bcrypt"
)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.New(st, logger, io.Discard)
	svc.BcryptCost = bcrypt.MinCost
	if err := seed.Run(ctx, svc); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(httpapi.New(svc, logger, httpapi.Options{}).Handler())
	t.Cleanup(ts.Close)
	return ts
}

type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func newClient(t *testing.T, ts *httptest.Server) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, base: ts.URL, http: &http.Client{Jar: jar}}
}

func login(t *testing.T, ts *httptest.Server, user string) *client {
	c := newClient(t, ts)
	if code, body := c.call("POST", "/api/auth/login", map[string]string{"userId": user, "password": seed.DefaultPassword}); code != 200 {
		t.Fatalf("login %s: %d %s", user, code, body)
	}
	return c
}

func (c *client) call(method, path string, body any) (int, string) {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.base+path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func (c *client) json(method, path string, body any, wantCode int, out any) {
	c.t.Helper()
	code, resp := c.call(method, path, body)
	if code != wantCode {
		c.t.Fatalf("%s %s: got %d want %d: %s", method, path, code, wantCode, resp)
	}
	if out != nil {
		if err := json.Unmarshal([]byte(resp), out); err != nil {
			c.t.Fatalf("decode %s: %v", resp, err)
		}
	}
}

func TestUnauthenticatedIs401(t *testing.T) {
	ts := newServer(t)
	c := newClient(t, ts)
	for _, p := range []string{"/api/auth/me", "/api/expenses", "/api/users", "/api/admin/audit-logs"} {
		if code, _ := c.call("GET", p, nil); code != http.StatusUnauthorized {
			t.Errorf("GET %s: got %d want 401", p, code)
		}
	}
	if code, _ := c.call("POST", "/api/auth/login", map[string]string{"userId": "employee1", "password": "bad"}); code != 401 {
		t.Errorf("bad login: %d", code)
	}
}

// TestAdminEndpointsForbiddenForNonAdmins calls every admin-only endpoint
// directly as Employee and Manager and expects 403 Forbidden.
func TestAdminEndpointsForbiddenForNonAdmins(t *testing.T) {
	ts := newServer(t)
	endpoints := []struct {
		method, path string
		body         any
	}{
		{"GET", "/api/users", nil},
		{"POST", "/api/users", map[string]any{"id": "hacker", "name": "h", "departmentId": 1, "role": "Admin", "password": "password123"}},
		{"PUT", "/api/users/employee1", map[string]any{"name": "h", "departmentId": 1, "role": "Admin"}},
		{"DELETE", "/api/users/employee2", nil},
		{"POST", "/api/accounts", map[string]any{"code": "X1", "name": "x"}},
		{"PUT", "/api/accounts/1", map[string]any{"code": "X1", "name": "x"}},
		{"DELETE", "/api/accounts/1", nil},
		{"GET", "/api/accounts?all=1", nil},
		{"POST", "/api/departments", map[string]any{"code": "X1", "name": "x"}},
		{"PUT", "/api/departments/1", map[string]any{"code": "X1", "name": "x"}},
		{"DELETE", "/api/departments/1", nil},
		{"GET", "/api/admin/audit-logs", nil},
		{"GET", "/api/admin/audit-logs/verify", nil},
		{"GET", "/api/admin/exports/settled.csv?from=2026-01-01&to=2026-12-31", nil},
		{"POST", "/api/admin/batch/reminders", nil},
		{"GET", "/api/admin/batch/reminders", nil},
		{"GET", "/api/expenses?scope=all", nil},
	}
	for _, user := range []string{"employee1", "manager1"} {
		c := login(t, ts, user)
		for _, e := range endpoints {
			if code, body := c.call(e.method, e.path, e.body); code != http.StatusForbidden {
				t.Errorf("%s %s %s: got %d want 403 (%s)", user, e.method, e.path, code, body)
			}
		}
	}
	// Sanity: the admin can use them.
	admin := login(t, ts, "admin1")
	admin.json("GET", "/api/users", nil, 200, nil)
	admin.json("GET", "/api/admin/audit-logs", nil, 200, nil)
}

type expense struct {
	ID             int64    `json:"id"`
	Status         string   `json:"status"`
	AllowedActions []string `json:"allowedActions"`
	History        []struct {
		Comment string `json:"comment"`
	} `json:"history"`
}

func TestApprovalFlowOverHTTP(t *testing.T) {
	ts := newServer(t)
	emp, mgr, mgr2, adm := login(t, ts, "employee1"), login(t, ts, "manager1"), login(t, ts, "manager2"), login(t, ts, "admin1")

	var accs []struct {
		ID   int64  `json:"id"`
		Code string `json:"code"`
	}
	emp.json("GET", "/api/accounts", nil, 200, &accs)

	var e expense
	emp.json("POST", "/api/expenses", map[string]any{"title": "HTTP", "items": []map[string]any{
		{"useDate": "2026-10-01", "accountId": accs[0].ID, "amount": 30000, "description": "短い"},
	}}, 201, &e)
	path := "/api/expenses/" + itoa(e.ID)

	// Business rule enforced by the API: 422 with field errors.
	code, body := emp.call("POST", path+"/submit", nil)
	if code != 422 || !strings.Contains(body, "items[0].description") {
		t.Fatalf("submit short description: %d %s", code, body)
	}
	emp.json("PUT", path, map[string]any{"title": "HTTP", "items": []map[string]any{
		{"useDate": "2026-10-01", "accountId": accs[0].ID, "amount": 30000, "description": strings.Repeat("あ", 50)},
	}}, 200, nil)
	emp.json("POST", path+"/submit", nil, 200, &e)
	if e.Status != "SUBMITTED" {
		t.Fatalf("status %s", e.Status)
	}

	// Other users cannot see it; other-department manager gets 403.
	if code, _ := login(t, ts, "employee2").call("GET", path, nil); code != 403 {
		t.Fatalf("employee2 view: %d", code)
	}
	if code, _ := mgr2.call("POST", path+"/approve", map[string]string{"comment": "ok"}); code != 403 {
		t.Fatalf("manager2 approve: %d", code)
	}
	// Employee cannot settle; admin cannot settle before manager approval (409).
	if code, _ := emp.call("POST", path+"/settle", map[string]string{"comment": "x"}); code != 403 {
		t.Fatalf("employee settle: %d", code)
	}
	if code, _ := adm.call("POST", path+"/settle", map[string]string{"comment": "x"}); code != 409 {
		t.Fatalf("admin settle submitted: %d", code)
	}
	// Comment is required for approval.
	if code, _ := mgr.call("POST", path+"/approve", map[string]string{"comment": ""}); code != 422 {
		t.Fatalf("approve without comment: %d", code)
	}
	mgr.json("POST", path+"/approve", map[string]string{"comment": "承認"}, 200, &e)
	adm.json("POST", path+"/reject", map[string]string{"comment": "領収書不備"}, 200, &e)
	if e.Status != "REJECTED" || len(e.AllowedActions) != 0 {
		t.Fatalf("after reject: %+v", e)
	}
	// REJECTED -> SETTLED is refused with 409 Conflict.
	if code, _ := adm.call("POST", path+"/settle", map[string]string{"comment": "x"}); code != 409 {
		t.Fatalf("settle rejected: %d", code)
	}
	if code, _ := adm.call("POST", path+"/bogus", nil); code != 404 {
		t.Fatalf("unknown action: %d", code)
	}
	emp.json("GET", path, nil, 200, &e)
	if len(e.History) != 3 || e.History[2].Comment != "領収書不備" {
		t.Fatalf("history: %+v", e.History)
	}
}

func TestCrossOriginMutationRejected(t *testing.T) {
	ts := newServer(t)
	c := login(t, ts, "employee1")
	req, _ := http.NewRequest("POST", ts.URL+"/api/expenses", strings.NewReader(`{"title":"x","items":[]}`))
	req.Header.Set("Origin", "https://evil.example")
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("cross-origin POST: %d", resp.StatusCode)
	}
}

func TestCSVExportAndAttachmentOverHTTP(t *testing.T) {
	ts := newServer(t)
	adm := login(t, ts, "admin1")
	code, body := adm.call("GET", "/api/admin/exports/settled.csv?from=2026-01-01&to=2099-12-31", nil)
	if code != 200 || !strings.Contains(body, "精算日,申請番号") || strings.Count(body, "\n") != 4 {
		t.Fatalf("csv: %d\n%s", code, body)
	}

	emp := login(t, ts, "employee1")
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "receipt.png")
	_, _ = fw.Write([]byte("fake-png"))
	_ = mw.Close()
	req, _ := http.NewRequest("POST", ts.URL+"/api/attachments", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := emp.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var att struct {
		ID int64 `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&att)
	_ = resp.Body.Close()
	if resp.StatusCode != 201 || att.ID == 0 {
		t.Fatalf("upload: %d", resp.StatusCode)
	}
	if code, body := emp.call("GET", "/api/attachments/"+itoa(att.ID), nil); code != 200 || body != "fake-png" {
		t.Fatalf("download: %d %s", code, body)
	}
	if code, _ := login(t, ts, "employee2").call("GET", "/api/attachments/"+itoa(att.ID), nil); code != 403 {
		t.Fatalf("foreign download: %d", code)
	}
}

func itoa(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
