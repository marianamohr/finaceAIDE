//go:build integration

package migrations

import (
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

func newSeedTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := newTestDB(t)
	if err := enableExtensions(db); err != nil {
		t.Fatalf("enable extensions: %v", err)
	}
	if err := db.AutoMigrate(&models.Account{}, &models.User{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

// uniqueSeedEmail evita colisão entre execuções e limpa o usuário ao final.
func uniqueSeedEmail(t *testing.T, db *gorm.DB) string {
	t.Helper()

	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("random email: %v", err)
	}
	email := "seed-" + hex.EncodeToString(buf) + "@example.test"

	t.Cleanup(func() {
		var user models.User
		if err := db.Where("email = ?", email).Take(&user).Error; err != nil {
			return
		}
		db.Unscoped().Delete(&models.User{}, "id = ?", user.ID)
		db.Unscoped().Delete(&models.Account{}, "id = ?", user.AccountID)
	})
	return email
}

func findSeedUser(t *testing.T, db *gorm.DB, email string) models.User {
	t.Helper()

	var user models.User
	if err := db.Where("email = ?", email).Take(&user).Error; err != nil {
		t.Fatalf("find user: %v", err)
	}
	return user
}

func TestEnsureSeedUser_CreatesMissingUser(t *testing.T) {
	db := newSeedTestDB(t)
	email := uniqueSeedEmail(t, db)

	if err := ensureSeedUser(db, email, "first-pass", "user"); err != nil {
		t.Fatalf("ensureSeedUser: %v", err)
	}

	user := findSeedUser(t, db, email)
	if user.Role != "user" || user.Status != "active" {
		t.Errorf("role/status = %q/%q, want user/active", user.Role, user.Status)
	}
	if err := auth.CheckPassword(user.PasswordHash, "first-pass"); err != nil {
		t.Errorf("password does not match: %v", err)
	}
}

func TestEnsureSeedUser_RotatesChangedPassword(t *testing.T) {
	db := newSeedTestDB(t)
	email := uniqueSeedEmail(t, db)

	if err := ensureSeedUser(db, email, "old-pass", "admin"); err != nil {
		t.Fatalf("seed old: %v", err)
	}
	if err := db.Model(&models.User{}).Where("email = ?", email).Update("status", "suspended").Error; err != nil {
		t.Fatalf("suspend: %v", err)
	}

	if err := ensureSeedUser(db, email, "new-pass", "admin"); err != nil {
		t.Fatalf("seed new: %v", err)
	}

	user := findSeedUser(t, db, email)
	if err := auth.CheckPassword(user.PasswordHash, "new-pass"); err != nil {
		t.Errorf("new password does not match: %v", err)
	}
	if err := auth.CheckPassword(user.PasswordHash, "old-pass"); err == nil {
		t.Error("old password still accepted")
	}
	if user.Status != "active" {
		t.Errorf("status = %q, want active", user.Status)
	}
}

func TestEnsureSeedUser_KeepsHashWhenPasswordUnchanged(t *testing.T) {
	db := newSeedTestDB(t)
	email := uniqueSeedEmail(t, db)

	if err := ensureSeedUser(db, email, "same-pass", "user"); err != nil {
		t.Fatalf("seed first: %v", err)
	}
	before := findSeedUser(t, db, email).PasswordHash

	if err := ensureSeedUser(db, email, "same-pass", "user"); err != nil {
		t.Fatalf("seed second: %v", err)
	}

	if after := findSeedUser(t, db, email).PasswordHash; after != before {
		t.Error("hash changed although password is the same")
	}
}
