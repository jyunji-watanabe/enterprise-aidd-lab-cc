// Package httpapi exposes the service layer as a JSON REST API and serves the SPA.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/domain"
	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/service"
	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/store"
)

// SessionCookie is the name of the session cookie.
const SessionCookie = "expense_session"

const maxJSONBody = 1 << 20

// Server is the HTTP API.
type Server struct {
	svc          *service.Service
	logger       *slog.Logger
	staticDir    string
	secureCookie bool
}

// Options configures the server.
type Options struct {
	// StaticDir, if set, is served as the SPA (with index.html fallback).
	StaticDir string
	// SecureCookie sets the Secure flag on the session cookie (enable behind HTTPS).
	SecureCookie bool
}

// New builds the server.
func New(svc *service.Service, logger *slog.Logger, opts Options) *Server {
	return &Server{svc: svc, logger: logger, staticDir: opts.StaticDir, secureCookie: opts.SecureCookie}
}

type ctxKey int

const (
	ctxUser ctxKey = iota
	ctxToken
)

type authedHandler func(w http.ResponseWriter, r *http.Request, actor domain.Actor) error

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	route := func(pattern string, h authedHandler) { mux.Handle(pattern, s.authed(h)) }

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	route("POST /api/auth/logout", s.handleLogout)
	route("GET /api/auth/me", s.handleMe)

	route("GET /api/departments", s.listDepartments)
	route("POST /api/departments", s.createDepartment)
	route("PUT /api/departments/{id}", s.updateDepartment)
	route("DELETE /api/departments/{id}", s.deleteDepartment)

	route("GET /api/accounts", s.listAccounts)
	route("POST /api/accounts", s.createAccount)
	route("PUT /api/accounts/{id}", s.updateAccount)
	route("DELETE /api/accounts/{id}", s.deleteAccount)

	route("GET /api/users", s.listUsers)
	route("POST /api/users", s.createUser)
	route("PUT /api/users/{id}", s.updateUser)
	route("DELETE /api/users/{id}", s.deleteUser)

	route("GET /api/expenses", s.listExpenses)
	route("POST /api/expenses", s.createExpense)
	route("GET /api/expenses/{id}", s.getExpense)
	route("PUT /api/expenses/{id}", s.updateExpense)
	route("POST /api/expenses/{id}/{action}", s.transitionExpense)

	route("POST /api/attachments", s.uploadAttachment)
	route("GET /api/attachments/{id}", s.downloadAttachment)

	route("GET /api/admin/audit-logs", s.listAuditLogs)
	route("GET /api/admin/audit-logs/verify", s.verifyAuditLogs)
	route("GET /api/admin/exports/settled.csv", s.exportSettledCSV)
	route("POST /api/admin/batch/reminders", s.runReminders)
	route("GET /api/admin/batch/reminders", s.listReminders)

	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not found", nil)
	})
	if s.staticDir != "" {
		mux.Handle("/", spaHandler(s.staticDir))
	}
	return s.recoverer(s.securityHeaders(s.requestLogger(mux)))
}

// ---- middleware ----

func (s *Server) authed(h authedHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(SessionCookie)
		token := ""
		if err == nil {
			token = c.Value
		}
		user, err := s.svc.Authenticate(r.Context(), token)
		if err != nil {
			s.writeServiceError(w, r, err)
			return
		}
		if r.Method != http.MethodGet && !sameOrigin(r) {
			writeError(w, http.StatusForbidden, "cross-origin request rejected", nil)
			return
		}
		ctx := context.WithValue(r.Context(), ctxUser, user)
		ctx = context.WithValue(ctx, ctxToken, token)
		if err := h(w, r.WithContext(ctx), service.ActorOf(user)); err != nil {
			s.writeServiceError(w, r, err)
		}
	})
}

// sameOrigin is a CSRF defence in addition to the SameSite=Strict cookie:
// state-changing requests must not carry a foreign Origin header.
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	o := strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://")
	return o == r.Host
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			s.logger.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "duration", time.Since(start).String())
		}
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				s.logger.Error("panic", "panic", p, "path", r.URL.Path)
				writeError(w, http.StatusInternalServerError, "internal error", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ---- helpers ----

type errorBody struct {
	Error  string              `json:"error"`
	Fields []domain.FieldError `json:"fields,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string, fields []domain.FieldError) {
	writeJSON(w, status, errorBody{Error: msg, Fields: fields})
}

func (s *Server) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		writeError(w, http.StatusUnprocessableEntity, "入力内容に誤りがあります", ve.Errors)
	case errors.Is(err, domain.ErrUnauthenticated):
		writeError(w, http.StatusUnauthorized, "認証が必要です", nil)
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusForbidden, "この操作を行う権限がありません", nil)
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "対象が見つかりません", nil)
	case errors.Is(err, domain.ErrInvalidTransition):
		writeError(w, http.StatusConflict, messageOr(err, domain.ErrInvalidTransition, "現在のステータスではこの操作はできません"), nil)
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, messageOr(err, domain.ErrConflict, "他の操作と競合しました。再読み込みしてください"), nil)
	case errors.Is(err, domain.ErrBadRequest):
		writeError(w, http.StatusBadRequest, messageOr(err, domain.ErrBadRequest, "リクエストが不正です"), nil)
	default:
		s.logger.Error("internal error", "err", err, "path", r.URL.Path)
		writeError(w, http.StatusInternalServerError, "内部エラーが発生しました", nil)
	}
}

// messageOr returns the detail text after "<sentinel>: " if present.
func messageOr(err, sentinel error, fallback string) string {
	msg := err.Error()
	if prefix := sentinel.Error() + ": "; strings.HasPrefix(msg, prefix) {
		return strings.TrimPrefix(msg, prefix)
	}
	return fallback
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxJSONBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return errors.Join(domain.ErrBadRequest, err)
	}
	return nil
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.Join(domain.ErrNotFound, err)
	}
	return id, nil
}

func meta(r *http.Request) service.Meta {
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ip = strings.TrimSpace(strings.Split(fwd, ",")[0]) + " (via " + ip + ")"
	}
	ua := r.UserAgent()
	if len(ua) > 300 {
		ua = ua[:300]
	}
	return service.Meta{IP: ip, UserAgent: ua}
}

func currentUser(r *http.Request) store.User {
	u, _ := r.Context().Value(ctxUser).(store.User)
	return u
}

// spaHandler serves static files and falls back to index.html for client-side routes.
// Files are read through os.DirFS, which rejects ".." and absolute paths, so a
// request can never reach outside dir.
func spaHandler(dir string) http.Handler {
	root := os.DirFS(dir)
	files := http.FileServerFS(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "."
		}
		if st, err := fs.Stat(root, name); err != nil || st.IsDir() {
			http.ServeFileFS(w, r, root, "index.html")
			return
		}
		files.ServeHTTP(w, r)
	})
}

// sessionCookie builds the session cookie (HttpOnly, SameSite=Strict).
// Secure is set when -secure-cookie is enabled or the request arrived over
// TLS; it cannot be unconditional because the local setup (`make run`,
// e2e tests) serves plain HTTP, where browsers would drop a Secure cookie.
func (s *Server) sessionCookie(r *http.Request, value string, maxAge int) *http.Cookie {
	return &http.Cookie{ //nolint:gosec // G124: Secure is configurable for plain-HTTP local use; see comment above.
		Name:     SessionCookie,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secureCookie || r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   maxAge,
	}
}
