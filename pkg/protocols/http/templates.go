package http

import "embed"

//go:embed templates/**/*
var htmlTemplateFS embed.FS
