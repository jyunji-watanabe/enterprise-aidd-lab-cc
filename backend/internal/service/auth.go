package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/domain"
	"github.com/jyunji-watanabe/enterprise-aidd-lab-cc/backend/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// dummyHash is compared against when the user does not exist, so that
// response time does not reveal which user IDs exist.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcrypt.MinCost)

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// HashPassword hashes a password with the configured cost.
func (s *Service) HashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), s.BcryptCost)
	return string(h), err
}

// ActorOf converts a user row into an authorization actor.
func ActorOf(u store.User) domain.Actor {
	return domain.Actor{UserID: u.ID, Name: u.Name, Role: u.Role, DepartmentID: u.DepartmentID}
}

// Login verifies credentials and creates a session. Both outcomes are audited.
func (s *Service) Login(ctx context.Context, userID, password string, meta Meta) (string, store.User, error) {
	user, err := store.GetUser(ctx, s.Store.DB, userID)
	hash := []byte(user.PasswordHash)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return "", store.User{}, err
		}
		hash = dummyHash
	}
	pwErr := bcrypt.CompareHashAndPassword(hash, []byte(password))
	if err != nil || pwErr != nil {
		reason := "invalid_password"
		if err != nil {
			reason = "unknown_user"
		}
		auditErr := s.Store.WithTx(ctx, func(tx store.DBTX) error {
			return s.audit(ctx, tx, meta, userID, AuditLoginFailure, "session", userID, nil, map[string]string{"reason": reason})
		})
		if auditErr != nil {
			return "", store.User{}, auditErr
		}
		return "", store.User{}, domain.ErrUnauthenticated
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", store.User{}, err
	}
	token := hex.EncodeToString(buf)
	now := s.Now()
	err = s.Store.WithTx(ctx, func(tx store.DBTX) error {
		if err := store.DeleteExpiredSessions(ctx, tx, store.FormatTime(now)); err != nil {
			return err
		}
		if err := store.InsertSession(ctx, tx, hashToken(token), user.ID, store.FormatTime(now.Add(s.SessionTTL)), store.FormatTime(now)); err != nil {
			return err
		}
		return s.audit(ctx, tx, meta, user.ID, AuditLoginSuccess, "session", user.ID, nil, map[string]string{"role": string(user.Role)})
	})
	if err != nil {
		return "", store.User{}, err
	}
	return token, user, nil
}

// Authenticate resolves a session token into the logged-in user.
func (s *Service) Authenticate(ctx context.Context, token string) (store.User, error) {
	if token == "" {
		return store.User{}, domain.ErrUnauthenticated
	}
	u, err := store.SessionUser(ctx, s.Store.DB, hashToken(token), s.now())
	if errors.Is(err, domain.ErrNotFound) {
		return store.User{}, domain.ErrUnauthenticated
	}
	return u, err
}

// Logout invalidates the session.
func (s *Service) Logout(ctx context.Context, token string, actor domain.Actor, meta Meta) error {
	return s.Store.WithTx(ctx, func(tx store.DBTX) error {
		if err := store.DeleteSession(ctx, tx, hashToken(token)); err != nil {
			return err
		}
		return s.audit(ctx, tx, meta, actor.UserID, AuditLogout, "session", actor.UserID, nil, nil)
	})
}
