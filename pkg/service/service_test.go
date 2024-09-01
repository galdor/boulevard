package service

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"testing"
	"time"

	"go.n16f.net/program"
)

const testCfgPath = "cfg/test.yaml"

var testService *Service

func TestMain(m *testing.M) {
	setTestDirectory()
	createTestACMEDatastore()

	initTestService()

	os.Exit(m.Run())
}

func setTestDirectory() {
	dirPath, err := program.ModuleDirectoryPath()
	if err != nil {
		abort("cannot obtain module directory path: %v", err)
	}

	if err := os.Chdir(dirPath); err != nil {
		abort("cannot change directory to %q: %v", dirPath, err)
	}
}

func createTestACMEDatastore() {
	// The datastore directory has to be a fixed path so that we can reference
	// it from cfg/test.yaml.

	dirPath := "/tmp/boulevard/acme"

	if err := os.RemoveAll(dirPath); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			abort("cannot delete directory %q: %v", dirPath, err)
		}
	}

	if err := os.MkdirAll(dirPath, 0700); err != nil {
		abort("cannot create directory %q: %w", dirPath, err)
	}
}

func initTestService() {
	var cfg ServiceCfg
	if err := cfg.Load(testCfgPath); err != nil {
		abort("cannot load configuration from %q: %v", testCfgPath, err)
	}

	cfg.BuildId = "test-" + time.Now().Format(time.RFC3339)
	cfg.ModuleInfo = DefaultModules

	service, err := NewService(cfg)
	if err != nil {
		abort("cannot create service: %v", err)
	}

	if err := service.Start(); err != nil {
		abort("cannot start service: %v", err)
	}
}

func abort(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func TestService(t *testing.T) {
	// This test just makes sure the service starts correctly
}
