package sybase

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"log"

	"github.com/hashicorp/errwrap"
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/builtin/logical/database/dbplugin"
	"github.com/hashicorp/vault/helper/dbtxn"
	"github.com/hashicorp/vault/helper/strutil"
	"github.com/hashicorp/vault/plugins"
	"github.com/hashicorp/vault/plugins/helper/database/credsutil"
	"github.com/hashicorp/vault/plugins/helper/database/dbutil"
	_ "github.com/rberlind/gofreetds"
)

const sybaseTypeName = "mssql"

var _ dbplugin.Database = &SYBASE{}

// SYBASE is an implementation of Database interface
type SYBASE struct {
	*SQLConnectionProducer
	credsutil.CredentialsProducer
}

func New() (interface{}, error) {
	db := new()
	// Wrap the plugin with middleware to sanitize errors
	dbType := dbplugin.NewDatabaseErrorSanitizerMiddleware(db, db.SecretValues)

	return dbType, nil
}

func new() *SYBASE {
	connProducer := &SQLConnectionProducer{}
	connProducer.Type = sybaseTypeName

	credsProducer := &credsutil.SQLCredentialsProducer{
		DisplayNameLen: 20,
		RoleNameLen:    20,
		UsernameLen:    30,
		Separator:      "_",
	}

	return &SYBASE{
		SQLConnectionProducer: connProducer,
		CredentialsProducer:   credsProducer,
	}
}

// Run instantiates a SYBASE object, and runs the RPC server for the plugin
func Run(apiTLSConfig *api.TLSConfig) error {
	dbType, err := New()
	if err != nil {
		return err
	}
	plugins.Serve(dbType.(dbplugin.Database), apiTLSConfig)

	return nil
}

// Type returns the TypeName for this backend
func (m *SYBASE) Type() (string, error) {
	return sybaseTypeName, nil
}

func (m *SYBASE) getConnection(ctx context.Context) (*sql.DB, error) {
	db, err := m.Connection(ctx)
	if err != nil {
		return nil, err
	}

	return db.(*sql.DB), nil
}

// CreateUser generates the username/password on the underlying SYBASE secret backend as instructed by
// the CreationStatement provided.
func (m *SYBASE) CreateUser(ctx context.Context, statements dbplugin.Statements, usernameConfig dbplugin.UsernameConfig, expiration time.Time) (username string, password string, err error) {
  log.Println("Calling CreateUser()")
	// Grab the lock
	m.Lock()
	defer m.Unlock()

	statements = dbutil.StatementCompatibilityHelper(statements)

	// Get the connection
	db, err := m.getConnection(ctx)
	if err != nil {
		return "", "", err
	}

	if len(statements.Creation) == 0 {
		return "", "", dbutil.ErrEmptyCreationStatement
	}

	username, err = m.GenerateUsername(usernameConfig)
	if err != nil {
		return "", "", err
	}

	password, err = m.GeneratePassword()
	if err != nil {
		return "", "", err
	}
	password = strings.Replace(password, "-", "_", -1)

	expirationStr, err := m.GenerateExpiration(expiration)
	if err != nil {
		return "", "", err
	}

	// Execute each query
	for _, stmt := range statements.Creation {
		for _, query := range strutil.ParseArbitraryStringSlice(stmt, ";") {
			query = strings.TrimSpace(query)
			if len(query) == 0 {
				continue
			}

			m := map[string]string{
				"name":       username,
				"password":   password,
				"expiration": expirationStr,
			}

      if err := dbtxn.ExecuteDBQuery(ctx, db, m, query); err != nil {
				return "", "", err
			}
		}
	}

	return username, password, nil
}

// RenewUser is not supported on SYBASE, so this is a no-op.
func (m *SYBASE) RenewUser(ctx context.Context, statements dbplugin.Statements, username string, expiration time.Time) error {
	// NOOP
	// But note that we could do this for Sybase if we convert
	// format of ttl back to number of days using:
	// alter login {{username}} modify password expiration {{expiration}}
	// probably first using time.Parse() followed by Until()
	// followed by Hours() / 24.
	return nil
}

// RevokeUser attempts to drop the specified user. It will first attempt to disable login,
// then drop the login and user from the
// database instance.
func (m *SYBASE) RevokeUser(ctx context.Context, statements dbplugin.Statements, username string) error {
	statements = dbutil.StatementCompatibilityHelper(statements)

	if len(statements.Revocation) == 0 {
		return m.revokeUserDefault(ctx, username)
	}

	// Get connection
	db, err := m.getConnection(ctx)
	if err != nil {
		return err
	}

	// Start a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Execute each query
	for _, stmt := range statements.Revocation {
		for _, query := range strutil.ParseArbitraryStringSlice(stmt, ";") {
			query = strings.TrimSpace(query)
			if len(query) == 0 {
				continue
			}

			m := map[string]string{
				"name": username,
			}
			if err := dbtxn.ExecuteTxQuery(ctx, tx, m, query); err != nil {
				return err
			}
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (m *SYBASE) revokeUserDefault(ctx context.Context, username string) error {
	// Get connection
	db, err := m.getConnection(ctx)
	if err != nil {
		return err
	}

	// First, disable server login
	lockLoginStmt, err := db.PrepareContext(ctx, fmt.Sprintf("master.dbo.sp_locklogin %s , \"lock\"", username))
	if err != nil {
		return errwrap.Wrapf("Could not prepare context for locking login: {{err}}", err)
	}
	defer lockLoginStmt.Close()
	if _, err := lockLoginStmt.ExecContext(ctx); err != nil {
		return errwrap.Wrapf("Could not execute context for locking login: {{err}}", err)
	}

	// Find the default database for the login
	// There should only be one
	var defaultDatabase string
	defaultDatabaseStmt, err := db.PrepareContext(ctx, fmt.Sprintf("SELECT dbname FROM master.dbo.syslogins WHERE name = '%s'", username))
	if err != nil {
		return errwrap.Wrapf("Could not prepare context for selecting dbname from syslogins: {{err}}", err)
	}
	defer defaultDatabaseStmt.Close()

	err = defaultDatabaseStmt.QueryRowContext(ctx).Scan(&defaultDatabase)
	switch {
	  case err == sql.ErrNoRows:
		  log.Println("No rows for defaultDatabase")
			return errwrap.Wrapf("No rows for defaultDatabase: {{err}}", err)
			//defaultDatabase = "vault"
	  case err != nil:
			log.Println("Some other error retrieving defaultDatabase")
		  return errwrap.Wrapf("Could not query context for selecting dbname from syslogins: {{err}}", err)
		default:
			log.Printf("Found defaultDatabase: '%s'", defaultDatabase)
	}

  dropUser := fmt.Sprintf(dropUserSQL, defaultDatabase, username, defaultDatabase, username)
	dropUserStmt, err := db.PrepareContext(ctx, dropUser)
	log.Printf("Invoking statement, '%s' to drop user from database '%s'", strings.Replace(dropUser, "\n", " ", -1), defaultDatabase)
	if err != nil {
		return errwrap.Wrapf("Could not prepare context for dropping user: {{err}}", err)
	}

	defer dropUserStmt.Close()
	if _, err = dropUserStmt.ExecContext(ctx); err != nil {
		return errwrap.Wrapf("could not drop user from database: {{err}}", err)
	} else {
		log.Printf("Dropped user '%s' from database '%s'", username, defaultDatabase)
	}

	// Drop this login
	dropLogin := fmt.Sprintf(dropLoginSQL, username, username)
	dropLoginStmt, err := db.PrepareContext(ctx, dropLogin)
	log.Printf("Invoking statement, '%s' to drop login '%s'", strings.Replace(dropLogin, "\n", " ", -1), username)
	if err != nil {
		return errwrap.Wrapf("Could not prepare context for dropping login: {{err}}", err)
	}

	defer dropLoginStmt.Close()
	if _, err = dropLoginStmt.ExecContext(ctx); err != nil {
		return errwrap.Wrapf("could not drop login from database: {{err}}", err)
	} else {
		log.Printf("Dropped login '%s'", username)
	}

	return nil
}

func (m *SYBASE) RotateRootCredentials(ctx context.Context, statements []string) (map[string]interface{}, error) {
	m.Lock()
	defer m.Unlock()

	if len(m.Username) == 0 || len(m.Password) == 0 {
		return nil, errors.New("username and password are required to rotate")
	}

	rotateStatements := statements
	if len(rotateStatements) == 0 {
		rotateStatements = []string{rotateRootCredentialsSQL}
	}

	db, err := m.getConnection(ctx)
	if err != nil {
		return nil, err
	}

	old_password := m.Password
	log.Println("Generating new password")
	password, err := m.GeneratePassword()
	if err != nil {
		return nil, err
	}
	password = strings.Replace(password, "-", "_", -1)

	for _, stmt := range rotateStatements {
		for _, query := range strutil.ParseArbitraryStringSlice(stmt, ";") {
			query = strings.TrimSpace(query)
			if len(query) == 0 {
				continue
			}

			m := map[string]string{
				"username":     m.Username,
				"old_password": old_password,
				"password":     password,
			}
			log.Printf("Executing query '%s'", query)
			if err := dbtxn.ExecuteDBQuery(ctx, db, m, query); err != nil {
				return nil, err
			}
		}
	}

	if err := db.Close(); err != nil {
		return nil, err
	}

	m.RawConfig["password"] = password
	return m.RawConfig, nil
}

const dropUserSQL = `
IF EXISTS
  (SELECT name
   FROM %s.dbo.sysusers
   WHERE name = '%s')
BEGIN
  execute %s.dbo.sp_dropuser %s
END
`

const dropLoginSQL = `
IF EXISTS
  (SELECT name
   FROM master.dbo.syslogins
   WHERE name = '%s')
BEGIN
  DROP LOGIN %s
END
`

const rotateRootCredentialsSQL = `
ALTER LOGIN {{username}} WITH PASSWORD {{old_password}} MODIFY PASSWORD IMMEDIATELY {{password}}
`
