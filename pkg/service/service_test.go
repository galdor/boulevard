package service

import (
	"net/url"
	"os"
	"testing"
	"time"

	"go.n16f.net/boulevard/pkg/httputils"
	"go.n16f.net/program"
)

const testCfgPath = "cfg/test.bcl"

var testService *Service

func TestMain(m *testing.M) {
	var status int

	func() {
		setTestDirectory()
		createTestACMEDatastore()

		initTestService()
		defer testService.Stop()

		status = m.Run()
	}()

	os.Exit(status)
}

func setTestDirectory() {
	dirPath, err := program.ModuleDirectoryPath()
	if err != nil {
		program.Abort("cannot obtain module directory path: %v", err)
	}

	if err := os.Chdir(dirPath); err != nil {
		program.Abort("cannot change directory to %q: %v", dirPath, err)
	}
}

func createTestACMEDatastore() {
	// The datastore directory has to be a fixed path so that we can reference
	// it from cfg/test.bcl.
	//
	// Usually we would delete it first to make sure we start from a clean
	// state, but the last thing we want is to have to regenerate TLS
	// certificates each time we run tests during development (5+ secondes even
	// with Pebble).

	dirPath := "/tmp/boulevard/acme"

	if err := os.MkdirAll(dirPath, 0700); err != nil {
		program.Abort("cannot create directory %q: %w", dirPath, err)
	}
}

func initTestService() {
	cfg := ServiceCfg{
		BuildId:      "test-" + time.Now().Format(time.RFC3339),
		ProtocolInfo: DefaultProtocols,
	}

	if err := cfg.Load(testCfgPath); err != nil {
		program.Abort("cannot load configuration from %q: %v", testCfgPath, err)
	}

	service, err := NewService(cfg)
	if err != nil {
		program.Abort("cannot create service: %v", err)
	}

	if err := service.Start(); err != nil {
		program.Abort("cannot start service: %v", err)
	}

	testService = service
}

func testHTTPClient(t *testing.T) *httputils.TestClient {
	baseURI := url.URL{
		Scheme: "http",
		Host:   "localhost:8080",
	}

	return httputils.NewTestClient(t, &baseURI)
}

func TestService(t *testing.T) {
	// This test just makes sure the service starts correctly
}
