package pkg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewAppRejectsUnavailableDatabase(t *testing.T) {
	config := filepath.Join(t.TempDir(), "invalid-db.yaml")
	data := []byte("nsf:\n  sbi_address: 127.0.0.1\n  sbi_port: 8000\n  db_type: unsupported\nlogger:\n  level: error\n")
	if err := os.WriteFile(config, data, 0600); err != nil {
		t.Fatal(err)
	}
	app, err := NewApp(config)
	if err == nil || app != nil {
		t.Fatal("unavailable database must prevent API startup")
	}
}
