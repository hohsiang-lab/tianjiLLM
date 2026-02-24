package db

import "embed"

//go:embed schema/*.up.sql
var SchemaFiles embed.FS
