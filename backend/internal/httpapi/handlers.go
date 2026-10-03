package httpapi

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/domain"
	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/service"
	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/store"
)

// ---- auth ----

type loginRequest struct {
	UserID   string `json:"userId"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	token, user, err := s.svc.Login(r.Context(), req.UserID, req.Password, meta(r))
	if err != nil {
		if errors.Is(err, domain.ErrUnauthenticated) {
			writeError(w, http.StatusUnauthorized, "ユーザーIDまたはパスワードが正しくありません", nil)
			return
		}
		s.writeServiceError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookie, Value: token, Path: "/", HttpOnly: true,
		Secure: s.secureCookie, SameSite: http.SameSiteStrictMode,
		MaxAge: int(s.svc.SessionTTL.Seconds()),
	})
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	token, _ := r.Context().Value(ctxToken).(string)
	if err := s.svc.Logout(r.Context(), token, actor, meta(r)); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: s.secureCookie, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type meResponse struct {
	store.User
	Permissions []domain.Permission `json:"permissions"`
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	perms := []domain.Permission{}
	for _, p := range []domain.Permission{domain.PermManageMasters, domain.PermViewAllExpenses, domain.PermViewAuditLog,
		domain.PermExportReports, domain.PermRunBatch, domain.PermApproveDept, domain.PermSettle} {
		if actor.Role.Can(p) {
			perms = append(perms, p)
		}
	}
	writeJSON(w, http.StatusOK, meResponse{User: currentUser(r), Permissions: perms})
	return nil
}

// ---- masters ----

func (s *Server) listDepartments(w http.ResponseWriter, r *http.Request, _ domain.Actor) error {
	out, err := s.svc.ListDepartments(r.Context())
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) createDepartment(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	var in service.DepartmentInput
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	out, err := s.svc.CreateDepartment(r.Context(), actor, in, meta(r))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

func (s *Server) updateDepartment(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	var in service.DepartmentInput
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	out, err := s.svc.UpdateDepartment(r.Context(), actor, id, in, meta(r))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) deleteDepartment(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := s.svc.DeleteDepartment(r.Context(), actor, id, meta(r)); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) listAccounts(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	out, err := s.svc.ListAccounts(r.Context(), actor, r.URL.Query().Get("all") == "1")
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) createAccount(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	var in service.AccountInput
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	out, err := s.svc.CreateAccount(r.Context(), actor, in, meta(r))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

func (s *Server) updateAccount(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	var in service.AccountInput
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	out, err := s.svc.UpdateAccount(r.Context(), actor, id, in, meta(r))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) deleteAccount(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := s.svc.DeleteAccount(r.Context(), actor, id, meta(r)); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	out, err := s.svc.ListUsers(r.Context(), actor)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	var in service.UserInput
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	out, err := s.svc.CreateUser(r.Context(), actor, in, meta(r))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	var in service.UserInput
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	out, err := s.svc.UpdateUser(r.Context(), actor, r.PathValue("id"), in, meta(r))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	if err := s.svc.DeleteUser(r.Context(), actor, r.PathValue("id"), meta(r)); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ---- expenses ----

func (s *Server) listExpenses(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	q := r.URL.Query()
	out, err := s.svc.ListExpenses(r.Context(), actor, q.Get("scope"), domain.Status(q.Get("status")))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) getExpense(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	out, err := s.svc.GetExpense(r.Context(), actor, id)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) createExpense(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	var in service.ExpenseInput
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	out, err := s.svc.CreateExpense(r.Context(), actor, in, meta(r))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

func (s *Server) updateExpense(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	var in service.ExpenseInput
	if err := decodeJSON(r, &in); err != nil {
		return err
	}
	out, err := s.svc.UpdateExpense(r.Context(), actor, id, in, meta(r))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

type transitionRequest struct {
	Comment string `json:"comment"`
}

func (s *Server) transitionExpense(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	action := domain.Action(r.PathValue("action"))
	if !action.Valid() {
		return domain.ErrNotFound
	}
	var req transitionRequest
	if r.ContentLength != 0 {
		if err := decodeJSON(r, &req); err != nil {
			return err
		}
	}
	out, err := s.svc.Transition(r.Context(), actor, id, action, req.Comment, meta(r))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// ---- attachments ----

func (s *Server) uploadAttachment(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	r.Body = http.MaxBytesReader(w, r.Body, service.MaxAttachmentSize+(1<<16))
	file, header, err := r.FormFile("file")
	if err != nil {
		return fmt.Errorf("%w: ファイルを選択してください（5MBまで）", domain.ErrBadRequest)
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("%w: ファイルの読み込みに失敗しました", domain.ErrBadRequest)
	}
	out, err := s.svc.UploadAttachment(r.Context(), actor, header.Filename, header.Header.Get("Content-Type"), data)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

func (s *Server) downloadAttachment(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	a, err := s.svc.GetAttachment(r.Context(), actor, id)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", a.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": a.Filename}))
	w.Header().Set("Content-Length", strconv.FormatInt(a.Size, 10))
	_, err = io.Copy(w, bytes.NewReader(a.Data))
	return err
}

// ---- admin ----

func (s *Server) listAuditLogs(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	logs, total, err := s.svc.ListAudit(r.Context(), actor, store.AuditFilter{
		Action: q.Get("action"), ActorID: q.Get("actorId"), ResourceType: q.Get("resourceType"),
		Limit: limit, Offset: max(offset, 0),
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": logs, "total": total})
	return nil
}

func (s *Server) verifyAuditLogs(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	out, err := s.svc.VerifyAudit(r.Context(), actor)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) exportSettledCSV(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	var buf bytes.Buffer
	if _, err := s.svc.ExportSettledCSV(r.Context(), actor, from, to, meta(r), &buf); err != nil {
		return err
	}
	name := fmt.Sprintf("settled_%s_%s.csv", from, to)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", name, url.PathEscape(name)))
	_, err := io.Copy(w, &buf)
	return err
}

func (s *Server) runReminders(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	out, err := s.svc.RunReminders(r.Context(), actor, meta(r))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) listReminders(w http.ResponseWriter, r *http.Request, actor domain.Actor) error {
	out, err := s.svc.ListReminders(r.Context(), actor)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}
