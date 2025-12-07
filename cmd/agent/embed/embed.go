package embed

import "embed"

//go:embed configs/*
var Configs embed.FS
