package service

import (
	"context"
	"fmt"

	"go.n16f.net/acme"
	"go.n16f.net/bcl"
)

type ACMECfg struct {
	DatastorePath       string
	Contact             []string
	DirectoryURI        string
	HTTPListenerAddress string
	HTTPUpstreamURI     string
	Pebble              bool
}

func (cfg *ACMECfg) Init(block *bcl.Element) {
	block.EntryValue("datastore_path", &cfg.DatastorePath)

	// TODO Validate email addresses
	if block.CheckEntryMinValues("contact", 1) {
		block.EntryValues("contact", &cfg.Contact)
	}

	block.MaybeEntryValue("pebble", &cfg.Pebble)

	if block := block.Block("http_challenge_solver"); block != nil {
		// TODO Validate address
		block.MaybeEntryValue("address", &cfg.HTTPListenerAddress)
		// TODO Validate URI
		block.EntryValue("upstream_uri", &cfg.HTTPUpstreamURI)
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
			Address:     cfg.HTTPListenerAddress,
			UpstreamURI: cfg.HTTPUpstreamURI,
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
