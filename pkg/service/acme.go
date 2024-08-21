package service

import (
	"context"
	"fmt"

	"go.n16f.net/acme"
	"go.n16f.net/ejson"
)

type ACMECfg struct {
	DatastorePath              string   `json:"datastore_path"`
	Contact                    []string `json:"contact"`
	DirectoryURI               string   `json:"directory_uri,omitempty"`
	HTTPChallengeSolverAddress string   `json:"http_challenge_solver_address,omitempty"`
	Pebble                     bool     `json:"pebble,omitempty"`
}

func (cfg *ACMECfg) ValidateJSON(v *ejson.Validator) {
	v.CheckStringNotEmpty("datastore_path", cfg.DatastorePath)

	v.WithChild("contact", func() {
		for i, address := range cfg.Contact {
			v.CheckStringNotEmpty(i, address)
		}
	})

	if cfg.HTTPChallengeSolverAddress != "" {
		v.CheckListenAddress("address", cfg.HTTPChallengeSolverAddress)
	}
}

func (s *Service) initACMEClient() error {
	cfg := s.Cfg.ACME
	if cfg == nil {
		return nil
	}

	logger := s.Log.Child("acme", nil)

	dataStore, err := acme.NewFileSystemDataStore(cfg.DatastorePath)
	if err != nil {
		return fmt.Errorf("cannot create file system datastore: %w", err)
	}

	contactURIs := make([]string, len(cfg.Contact))
	for i, address := range cfg.Contact {
		contactURIs[i] = "mailto:" + address
	}

	clientCfg := acme.ClientCfg{
		Log:       logger,
		DataStore: dataStore,

		UserAgent:   s.httpUserAgent,
		ContactURIs: contactURIs,

		HTTPChallengeSolver: &acme.HTTPChallengeSolverCfg{
			Address: cfg.HTTPChallengeSolverAddress,
		},
	}

	if cfg.Pebble {
		if clientCfg.DirectoryURI == "" {
			clientCfg.DirectoryURI = acme.PebbleDirectoryURI
		}

		if clientCfg.HTTPChallengeSolver.Address == "" {
			clientCfg.HTTPChallengeSolver.Address =
				acme.PebbleHTTPChallengeSolverAddress
		}

		clientCfg.HTTPClient =
			acme.NewHTTPClient(acme.PebbleCACertificatePool())
	} else {
		if clientCfg.DirectoryURI == "" {
			clientCfg.DirectoryURI = acme.LetsEncryptDirectoryURI
		}

		if clientCfg.HTTPChallengeSolver.Address == "" {
			clientCfg.HTTPChallengeSolver.Address = "0.0.0.0:80"
		}
	}

	client, err := acme.NewClient(clientCfg)
	if err != nil {
		return fmt.Errorf("cannot create ACME client: %w", err)
	}
	s.acmeClient = client

	return nil
}

func (s *Service) startACMEClient() error {
	if s.acmeClient == nil {
		return nil
	}

	s.Log.Debug(1, "starting ACME client")

	ctx := context.Background()

	if err := s.acmeClient.Start(ctx); err != nil {
		return fmt.Errorf("cannot start ACME client: %w", err)
	}

	return nil
}

func (s *Service) stopACMEClient() {
	if s.acmeClient == nil {
		return
	}

	s.Log.Debug(1, "stopping ACME client")

	s.acmeClient.Stop()
}
