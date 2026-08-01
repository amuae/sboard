package database

import (
	"bytes"
	"log"
	"strings"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestDatabaseLoggerIgnoresRecordNotFound(t *testing.T) {
	var output bytes.Buffer
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: newDatabaseLogger(log.New(&output, "", 0)),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&SystemConfig{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	var config SystemConfig
	err = db.Where("key = ?", "missing").First(&config).Error
	if err != gorm.ErrRecordNotFound {
		t.Fatalf("First() error = %v, want record not found", err)
	}
	if strings.Contains(output.String(), "record not found") {
		t.Fatalf("expected record-not-found query to be silent, got %q", output.String())
	}
}

func TestEnsureExtraUUIDGeneratesAndPersistsMissingSlot(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:extra-uuid-test?mode=memory&cache=shared"), &gorm.Config{
		Logger: newDatabaseLogger(log.New(&bytes.Buffer{}, "", 0)),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&ProxyUser{}, &UserExtraUUID{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	previousDB := DB
	SetDB(db)
	t.Cleanup(func() { SetDB(previousDB) })

	user := ProxyUser{Name: "test", UUID: "550e8400-e29b-41d4-a716-446655440000", Enabled: 1}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	got, err := user.EnsureExtraUUID(1)
	if err != nil {
		t.Fatalf("EnsureExtraUUID() error = %v", err)
	}
	if got == "" {
		t.Fatal("EnsureExtraUUID() returned an empty UUID")
	}

	var count int64
	db.Model(&UserExtraUUID{}).Where("user_id = ? AND slot = ? AND uuid = ?", user.ID, 1, got).Count(&count)
	if count != 1 {
		t.Fatalf("persisted UUID count = %d, want 1", count)
	}
	var persisted ProxyUser
	if err := db.First(&persisted, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if persisted.UUID1 != got {
		t.Fatalf("legacy UUID1 = %q, want %q", persisted.UUID1, got)
	}

	gotAgain, err := user.EnsureExtraUUID(1)
	if err != nil {
		t.Fatalf("EnsureExtraUUID() second call error = %v", err)
	}
	if gotAgain != got {
		t.Fatalf("second UUID = %q, want original %q", gotAgain, got)
	}
}

func TestEnsureExtraUUIDConvergesUnderConcurrentSubscriptions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:extra-uuid-concurrency-test?mode=memory&cache=shared"), &gorm.Config{
		Logger: newDatabaseLogger(log.New(&bytes.Buffer{}, "", 0)),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&ProxyUser{}, &UserExtraUUID{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	previousDB := DB
	SetDB(db)
	t.Cleanup(func() { SetDB(previousDB) })

	user := ProxyUser{Name: "concurrent", UUID: "750e8400-e29b-41d4-a716-446655440000", Enabled: 1}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	const workers = 8
	values := make(chan string, workers)
	errors := make(chan error, workers)
	var group sync.WaitGroup
	group.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer group.Done()
			var loaded ProxyUser
			if err := db.First(&loaded, user.ID).Error; err != nil {
				errors <- err
				return
			}
			value, err := loaded.EnsureExtraUUID(1)
			if err != nil {
				errors <- err
				return
			}
			values <- value
		}()
	}
	group.Wait()
	close(values)
	close(errors)
	for err := range errors {
		t.Fatalf("concurrent EnsureExtraUUID() error = %v", err)
	}

	var expected string
	for value := range values {
		if expected == "" {
			expected = value
		}
		if value != expected {
			t.Fatalf("concurrent UUID = %q, want %q", value, expected)
		}
	}
	var count int64
	db.Model(&UserExtraUUID{}).Where("user_id = ? AND slot = ?", user.ID, 1).Count(&count)
	if count != 1 {
		t.Fatalf("normalized UUID count = %d, want 1", count)
	}
}

func TestMigrateUserExtraUUIDsReconcilesExistingRecords(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:extra-uuid-migration-test?mode=memory&cache=shared"), &gorm.Config{
		Logger: newDatabaseLogger(log.New(&bytes.Buffer{}, "", 0)),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&ProxyUser{}, &UserExtraUUID{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	previousDB := DB
	SetDB(db)
	t.Cleanup(func() { SetDB(previousDB) })

	user := ProxyUser{Name: "legacy", UUID: "650e8400-e29b-41d4-a716-446655440000", Enabled: 1}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	existing := UserExtraUUID{UserID: user.ID, Slot: 1, UUID: "normalized-slot-1"}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create normalized UUID: %v", err)
	}

	if err := migrateUserExtraUUIDs(); err != nil {
		t.Fatalf("migrateUserExtraUUIDs() error = %v", err)
	}

	var persisted ProxyUser
	if err := db.First(&persisted, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if persisted.UUID1 != existing.UUID {
		t.Fatalf("legacy UUID1 = %q, want normalized value %q", persisted.UUID1, existing.UUID)
	}
	var records []UserExtraUUID
	if err := db.Where("user_id = ?", user.ID).Find(&records).Error; err != nil {
		t.Fatalf("reload normalized UUIDs: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("normalized record count = %d, want 1", len(records))
	}

	// A second startup migration must preserve the same record and not rotate
	// or duplicate it.
	if err := migrateUserExtraUUIDs(); err != nil {
		t.Fatalf("second migrateUserExtraUUIDs() error = %v", err)
	}
	var count int64
	db.Model(&UserExtraUUID{}).Where("user_id = ? AND slot = ?", user.ID, 1).Count(&count)
	if count != 1 {
		t.Fatalf("normalized slot count after second migration = %d, want 1", count)
	}
}
