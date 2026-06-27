package webui

import (
	"embed"
)

//go:embed static/*
var StaticFS embed.FS

//go:embed templates/*.html
var TemplateFS embed.FS
