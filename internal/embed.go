package ui

import "embed"

//go:embed web/templates/* web/static/*
var StaticFS embed.FS
