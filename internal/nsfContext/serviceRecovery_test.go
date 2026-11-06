package nsfContext_test

import (
	"fmt"
	"nsf/internal/db"
	"nsf/internal/nsfContext"
	"nsf/logger"
	"path/filepath"
	"sync"
	"testing"

	loggergoUtil "github.com/Alonza0314/logger-go/v2/util"
)

func openServiceTestContext(t *testing.T) *nsfContext.NsfContext {
	t.Helper()
	log := logger.NewNsfLogger(loggergoUtil.LogLevelString("error"))
	ctx := nsfContext.NewNsfContext(&nsfContext.NsfContextIE{DbType: "bbolt", NsfLogger: &log})
	if ctx == nil {
		t.Fatal("failed to open service test context")
	}
	return ctx
}

func TestServiceRecoveryAndPartialDeregistration(t *testing.T) {
	previousPath := db.NSF_DB_PATH
	db.NSF_DB_PATH = filepath.Join(t.TempDir(), "recovery.db")
	defer func() { db.NSF_DB_PATH = previousPath }()
	ctx := openServiceTestContext(t)
	for _, nf := range []string{"NF-A", "NF-B"} {
		if err := ctx.RegisterServices(nf, "127.0.0.1", 8080, []string{"shared", nf}); err != nil {
			t.Fatal(err)
		}
	}
	ctx.Release()

	ctx = openServiceTestContext(t)
	defer ctx.Release()
	name, _, _, err := ctx.DiscoverService("shared")
	if err != nil || name != "NF-A" {
		t.Fatalf("recovered discovery = %q, %v; want NF-A", name, err)
	}
	if err := ctx.DeregisterServices("NF-A", []string{"shared"}); err != nil {
		t.Fatal(err)
	}
	name, _, _, err = ctx.DiscoverService("shared")
	if err != nil || name != "NF-B" {
		t.Fatalf("remaining provider = %q, %v; want NF-B", name, err)
	}
	if _, _, _, err := ctx.DiscoverService("NF-A"); err != nil {
		t.Fatalf("partial deregistration removed an unrelated service: %v", err)
	}
	if err := ctx.DeregisterServices("NF-B", []string{"shared", "NF-B"}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ctx.DiscoverService("shared"); err == nil {
		t.Fatal("deregistered service is still discoverable")
	}
	if _, _, _, err := ctx.DiscoverService("never-registered"); err == nil {
		t.Fatal("unknown service must return an error")
	}
}

func TestConcurrentServiceRegistration(t *testing.T) {
	previousPath := db.NSF_DB_PATH
	db.NSF_DB_PATH = filepath.Join(t.TempDir(), "concurrent.db")
	defer func() { db.NSF_DB_PATH = previousPath }()
	ctx := openServiceTestContext(t)
	defer ctx.Release()

	var workers sync.WaitGroup
	for i := range 16 {
		workers.Go(func() {
			service := fmt.Sprintf("service-%d", i)
			if err := ctx.RegisterServices("NF", "127.0.0.1", 8080, []string{service, service}); err != nil {
				t.Error(err)
			}
			if _, _, _, err := ctx.DiscoverService(service); err != nil {
				t.Error(err)
			}
		})
	}
	workers.Wait()
	for i := range 16 {
		if _, _, _, err := ctx.DiscoverService(fmt.Sprintf("service-%d", i)); err != nil {
			t.Errorf("concurrent registration lost service %d: %v", i, err)
		}
	}
}
