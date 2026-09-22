package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/buscompany/bus_fleet/internal/auth"
	"github.com/buscompany/bus_fleet/internal/domain"
)

// testParams keep the tests fast; production uses auth.DefaultParams().
func testParams() auth.Params {
	p := auth.DefaultParams()
	p.Memory = 8 * 1024
	p.Iterations = 1
	return p
}

func TestHashPassword(t *testing.T) {
	p := testParams()
	hash, err := auth.HashPassword("correct horse battery staple", p)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	// The DB CHECK on users.password_hash requires this prefix.
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("hash %q is not a PHC argon2id string", hash)
	}
	if strings.Contains(hash, "correct horse") {
		t.Fatal("hash contains the password")
	}

	again, err := auth.HashPassword("correct horse battery staple", p)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == again {
		t.Fatal("two hashes of the same password are identical: salt is not random")
	}
}

func TestVerifyPassword(t *testing.T) {
	p := testParams()
	hash, err := auth.HashPassword("s3cret-pass", p)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	tests := []struct {
		name     string
		password string
		hash     string
		want     bool
		wantErr  bool
	}{
		{name: "correct password", password: "s3cret-pass", hash: hash, want: true},
		{name: "wrong password", password: "s3cret-pas", hash: hash, want: false},
		{name: "empty password", password: "", hash: hash, want: false},
		{name: "case differs", password: "S3cret-Pass", hash: hash, want: false},
		{name: "plaintext stored", password: "s3cret-pass", hash: "s3cret-pass", want: false, wantErr: true},
		{name: "bcrypt stored", password: "s3cret-pass", hash: "$2a$10$abcdefghijklmnopqrstuv", want: false, wantErr: true},
		{name: "truncated hash", password: "s3cret-pass", hash: hash[:len(hash)-4], want: false},
		{name: "empty hash", password: "s3cret-pass", hash: "", want: false, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := auth.VerifyPassword(tt.password, tt.hash)
			if tt.wantErr && err == nil {
				t.Fatal("expected an error for a malformed hash")
			}
			if got != tt.want {
				t.Fatalf("VerifyPassword = %v, want %v (err=%v)", got, tt.want, err)
			}
		})
	}
}

func TestDummyHashIsVerifiable(t *testing.T) {
	// The login handler verifies against this when no user matches; it must be a
	// well-formed hash so the work (and therefore the timing) is the same.
	hash, err := auth.DummyHash(testParams())
	if err != nil {
		t.Fatalf("DummyHash: %v", err)
	}
	ok, err := auth.VerifyPassword("anything", hash)
	if err != nil {
		t.Fatalf("VerifyPassword on dummy hash: %v", err)
	}
	if ok {
		t.Fatal("dummy hash accepted a password")
	}
}

func TestCSRFTokens(t *testing.T) {
	secretA := []byte("0123456789abcdef0123456789abcdef")
	secretB := []byte("fedcba9876543210fedcba9876543210")

	tokenA := auth.SessionCSRFToken(secretA)
	if tokenA == "" {
		t.Fatal("empty token")
	}
	if tokenA != auth.SessionCSRFToken(secretA) {
		t.Fatal("token is not stable for the same session secret")
	}
	if tokenA == auth.SessionCSRFToken(secretB) {
		t.Fatal("different session secrets produced the same token")
	}
	// Anonymous and session tokens are domain-separated, so one context's token can
	// never be replayed in the other.
	if tokenA == auth.AnonCSRFToken(secretA, []byte("nonce")) {
		t.Fatal("anon and session tokens collide for the same key")
	}

	nonce, err := auth.NewCSRFNonce()
	if err != nil {
		t.Fatalf("NewCSRFNonce: %v", err)
	}
	if nonce == "" {
		t.Fatal("empty nonce")
	}
	anon := auth.AnonCSRFToken(secretA, []byte(nonce))
	if anon == auth.AnonCSRFToken(secretA, []byte(nonce+"x")) {
		t.Fatal("token does not depend on the nonce")
	}

	tests := []struct {
		name               string
		expected, provided string
		want               bool
	}{
		{name: "match", expected: tokenA, provided: tokenA, want: true},
		{name: "mismatch", expected: tokenA, provided: auth.SessionCSRFToken(secretB), want: false},
		{name: "missing submitted", expected: tokenA, provided: "", want: false},
		{name: "missing expected", expected: "", provided: tokenA, want: false},
		{name: "both empty", expected: "", provided: "", want: false},
		{name: "prefix only", expected: tokenA, provided: tokenA[:8], want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := auth.ValidCSRFToken(tt.expected, tt.provided); got != tt.want {
				t.Fatalf("ValidCSRFToken = %v, want %v", got, tt.want)
			}
		})
	}
}

// fakeStore records what the manager asks of the database.
type fakeStore struct {
	created        domain.Session
	identity       domain.Identity
	getErr         error
	gotIdleCutoff  time.Time
	deletedHash    []byte
	deletedUserID  int64
	deleteCalled   bool
	deleteAllCalls int
}

func (f *fakeStore) CreateSession(_ context.Context, s domain.Session) error {
	f.created = s
	return nil
}

func (f *fakeStore) GetAndTouchSession(_ context.Context, _ []byte, idleCutoff time.Time) (domain.Identity, error) {
	f.gotIdleCutoff = idleCutoff
	if f.getErr != nil {
		return domain.Identity{}, f.getErr
	}
	return f.identity, nil
}

func (f *fakeStore) DeleteSession(_ context.Context, tokenHash []byte) error {
	f.deleteCalled = true
	f.deletedHash = tokenHash
	return nil
}

func (f *fakeStore) DeleteSessionsForUser(_ context.Context, userID int64) error {
	f.deleteAllCalls++
	f.deletedUserID = userID
	return nil
}

func (f *fakeStore) DeleteExpiredSessions(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

func TestManagerCreateStoresOnlyTheHash(t *testing.T) {
	store := &fakeStore{}
	m := auth.NewManager(store, time.Hour, 8*time.Hour)

	token, expiresAt, err := m.Create(context.Background(), 42)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if token == "" {
		t.Fatal("empty token")
	}
	if string(store.created.TokenHash) == token {
		t.Fatal("the raw token was stored; only its hash may be persisted")
	}
	if len(store.created.TokenHash) != 32 {
		t.Fatalf("token hash is %d bytes, want 32 (sha256)", len(store.created.TokenHash))
	}
	if string(store.created.TokenHash) != string(auth.HashToken(token)) {
		t.Fatal("stored hash does not match HashToken(token)")
	}
	if len(store.created.CSRFSecret) != 32 {
		t.Fatalf("csrf secret is %d bytes, want 32", len(store.created.CSRFSecret))
	}
	if store.created.UserID != 42 {
		t.Fatalf("stored user id %d, want 42", store.created.UserID)
	}
	if got := time.Until(expiresAt); got > 8*time.Hour+time.Minute || got < 7*time.Hour {
		t.Fatalf("absolute expiry %v is not ~8h", got)
	}

	// A second session for the same user must not reuse the token.
	token2, _, err := m.Create(context.Background(), 42)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if token == token2 {
		t.Fatal("two sessions share a token")
	}
}

func TestManagerResolve(t *testing.T) {
	store := &fakeStore{identity: domain.Identity{UserID: 7, Role: domain.RoleDriver}}
	m := auth.NewManager(store, 30*time.Minute, 8*time.Hour)

	token, _, err := m.Create(context.Background(), 7)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	id, err := m.Resolve(context.Background(), token)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if id.UserID != 7 {
		t.Fatalf("resolved user %d, want 7", id.UserID)
	}
	// The idle cutoff must be handed to SQL, where expiry is actually enforced.
	if drift := time.Since(store.gotIdleCutoff.Add(30 * time.Minute)); drift > time.Minute || drift < -time.Minute {
		t.Fatalf("idle cutoff %v is not now-30m", store.gotIdleCutoff)
	}

	t.Run("unknown or expired session is unauthorized", func(t *testing.T) {
		store.getErr = domain.ErrNotFound
		if _, err := m.Resolve(context.Background(), token); !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("got %v, want ErrUnauthorized", err)
		}
	})

	t.Run("malformed tokens never reach the database", func(t *testing.T) {
		probe := &fakeStore{getErr: errors.New("database must not be queried")}
		pm := auth.NewManager(probe, time.Hour, time.Hour)
		for _, bad := range []string{"", "x", strings.Repeat("A", 100), "!!!!not-base64!!!!"} {
			if _, err := pm.Resolve(context.Background(), bad); !errors.Is(err, domain.ErrUnauthorized) {
				t.Fatalf("token %q: got %v, want ErrUnauthorized", bad, err)
			}
		}
		if !probe.gotIdleCutoff.IsZero() {
			t.Fatal("a malformed token was sent to the store")
		}
	})
}

func TestManagerDestroy(t *testing.T) {
	store := &fakeStore{}
	m := auth.NewManager(store, time.Hour, 8*time.Hour)

	token, _, err := m.Create(context.Background(), 1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := m.Destroy(context.Background(), token); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if !store.deleteCalled {
		t.Fatal("logout did not delete the session server-side")
	}
	if string(store.deletedHash) != string(auth.HashToken(token)) {
		t.Fatal("logout deleted the wrong session")
	}

	if err := m.DestroyAllForUser(context.Background(), 5); err != nil {
		t.Fatalf("DestroyAllForUser: %v", err)
	}
	if store.deleteAllCalls != 1 || store.deletedUserID != 5 {
		t.Fatalf("DestroyAllForUser did not delete user 5's sessions (calls=%d, user=%d)",
			store.deleteAllCalls, store.deletedUserID)
	}
}
