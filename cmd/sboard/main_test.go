package main

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/sboard-go/sboard/internal/database"
	"gorm.io/gorm"
)

func TestEnablePasswordLoginUpdatesDatabaseSetting(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&database.SystemConfig{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	database.SetDB(db)

	if err := database.SetDisablePasswordLogin(true); err != nil {
		t.Fatalf("disable password login: %v", err)
	}
	if !database.GetDisablePasswordLogin() {
		t.Fatal("password login was not disabled during test setup")
	}

	if err := enablePasswordLogin(); err != nil {
		t.Fatalf("enablePasswordLogin() error = %v", err)
	}
	if database.GetDisablePasswordLogin() {
		t.Fatal("enablePasswordLogin() did not update the database setting")
	}
}
