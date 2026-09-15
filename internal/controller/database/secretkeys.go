package database

// Canonical data keys for the Secrets the database adapters write. Shared by
// the ManagedDatabase and ManagedDatabaseUser adapters and their tests so the
// connection and credentials Secrets use one spelling.
const (
	SecretKeyURI      = "uri"
	SecretKeyHost     = "host"
	SecretKeyPort     = "port"
	SecretKeyUser     = "user"
	SecretKeyPassword = "password"
	SecretKeyDBName   = "dbname"
	SecretKeySSLMode  = "sslmode"
	SecretKeyUsername = "username"
)
