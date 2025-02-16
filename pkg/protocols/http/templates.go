package http

import "embed"

//go:embed templates/**/*
var templateFS embed.FS
