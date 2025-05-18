package service

import (
	"context"
	"fmt"

	"go.n16f.net/acme/pkg/acme"
	"go.n16f.net/bcl"
	"go.n16f.net/boulevard/pkg/httputils"
	"go.n16f.net/boulevard/pkg/netutils"
)

type ACMECfg struct {
	DatastorePath       string
	Contacts            []string
	DirectoryURI        string
	HTTPListenerAddress string
	HTTPUpstreamURI     string
	Pebble              bool
}

func (cfg *ACMECfg) ReadBCLElement(block *bcl.Element) error {
	block.EntryValues("datastore_path", &cfg.DatastorePath)

	for _, entry := range block.FindEntries("contact") {
		var contact string
		entry.Values(
			bcl.WithValueValidation(&contact, netutils.ValidateBCLEmailAddress))
		cfg.Contacts = append(cfg.Contacts, contact)
	}

	block.MaybeEntryValues("pebble", &cfg.Pebble)

	if block := block.FindBlock("http_challenge_solver"); block != nil {
		block.MaybeEntryValues("address",
			bcl.WithValueValidation(&cfg.HTTPListenerAddress,
				netutils.ValidateBCLAddress))

		block.EntryValues("upstream_uri",
			bcl.WithValueValidation(&cfg.HTTPUpstreamURI,
				httputils.ValidateBCLHTTPURI))
	}

	return nil
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

	contactURIs := make([]string, len(cfg.Contacts))
	for i, address := range cfg.Contacts {
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
