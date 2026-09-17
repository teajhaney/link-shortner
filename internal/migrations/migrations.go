// Package migrations embeds the SQL schema files so they are compiled into the
// binary and can be applied at startup instead of by hand.
package migrations

import (
	"embed"
	"io/fs"
)

//go:embed *.sql
var sqlFiles embed.FS

// Files is the embedded migration directory. Entries are the .sql files in
// this package, addressable by their bare filename ("0001_init.sql").
var Files fs.FS = sqlFiles
