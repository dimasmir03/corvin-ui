package ui

import "embed"

//go:embed templates/* static/*
var StaticFS embed.FS
