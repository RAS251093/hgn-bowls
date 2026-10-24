package db

import "io/fs"

// migrationsFS returns the embedded migrations rooted at their directory.
func migrationsFS() fs.FS {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		panic(err) // can't happen: the directory is embedded at build time
	}
	return sub
}
