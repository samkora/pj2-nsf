package sbi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"nsf/internal/db"
	"nsf/logger"
	"nsf/model"
	"path/filepath"
	"testing"

	loggergoUtil "github.com/Alonza0314/logger-go/v2/util"
	"github.com/gin-gonic/gin"
)

func TestServiceHTTPFlow(t *testing.T) {
	previousPath := db.NSF_DB_PATH
	db.NSF_DB_PATH = filepath.Join(t.TempDir(), "http.db")
	defer func() { db.NSF_DB_PATH = previousPath }()
	log := logger.NewNsfLogger(loggergoUtil.LogLevelString("error"))
	s := NewServer(&ServerIE{DbType: "bbolt", NsfLogger: &log})
	defer s.Processor.Release()
	gin.SetMode(gin.TestMode)
	s.router = gin.New()
	addService(s)

	request := func(method, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		s.router.ServeHTTP(response, req)
		if response.Code != status {
			t.Fatalf("%s %s = %d %s; want %d", method, path, response.Code, response.Body, status)
		}
		return response
	}
	request(http.MethodGet, "/sbi/heartbeat", "", http.StatusNoContent)
	request(http.MethodGet, "/sbi/nnsf-disc", `{"service":"missing"}`, http.StatusNotFound)
	response := request(http.MethodPost, "/sbi/nnsf-nfm",
		`{"nf_name":"NF","sbi_address":"127.0.0.2","sbi_port":8081,"services":["one","two"]}`, http.StatusOK)
	var registration model.NnsfNfmPostResponse
	if err := json.Unmarshal(response.Body.Bytes(), &registration); err != nil {
		t.Fatal(err)
	}
	if registration.ServiceUuid == "" || registration.ServiceToken == "" {
		t.Fatal("registration must return UUID and token")
	}
	response = request(http.MethodGet, "/sbi/nnsf-disc/group", `{"services":["two","one"]}`, http.StatusOK)
	var group model.NnsfDiscGroupGetResponse
	if err := json.Unmarshal(response.Body.Bytes(), &group); err != nil {
		t.Fatal(err)
	}
	if len(group.Services) != 2 || group.Services[0].Service != "two" || group.Services[1].Service != "one" {
		t.Fatalf("unexpected group discovery: %+v", group)
	}
	for _, service := range group.Services {
		if service.NfName != "NF" || service.SbiAddress != "127.0.0.2" || service.SbiPort != 8081 {
			t.Fatalf("incorrect service endpoint: %+v", service)
		}
	}
	request(http.MethodDelete, "/sbi/nnsf-nfm", `{"nf_name":"NF","services":["one"]}`, http.StatusNoContent)
	request(http.MethodGet, "/sbi/nnsf-disc", `{"service":"one"}`, http.StatusNotFound)
	request(http.MethodGet, "/sbi/nnsf-disc", `{"service":"two"}`, http.StatusOK)
	request(http.MethodGet, "/sbi/nnsf-disc", `{`, http.StatusBadRequest)
}
