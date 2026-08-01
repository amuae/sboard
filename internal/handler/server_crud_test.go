package handler

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/sboard-go/sboard/internal/database"
	"gorm.io/gorm"
)

func TestHandleCreateServerPreservesRequestedSSHPort(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:create-server-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&database.Server{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	previousDB := database.DB
	database.SetDB(db)
	t.Cleanup(func() { database.SetDB(previousDB) })

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("POST", "/api/servers", bytes.NewBufferString(`{
		"name":"custom-port",
		"port":22022,
		"enabled":1
	}`))
	context.Request.Header.Set("Content-Type", "application/json")

	(&Server{}).handleCreateServer(context)
	if recorder.Code != 200 {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data database.Server `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Port != 22022 {
		t.Fatalf("created server port = %d, want 22022", response.Data.Port)
	}
}
