package sybase

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/vault/builtin/logical/database/dbplugin"
)

var (
	testMSQLImagePull sync.Once
)

func TestSYBASE_Initialize(t *testing.T) {
	if os.Getenv("SYBASE_URL") == "" || os.Getenv("VAULT_ACC") != "1" {
		return
	}
	connURL := os.Getenv("SYBASE_URL")

	connectionDetails := map[string]interface{}{
		"connection_url": connURL,
	}

	db := new()
	_, err := db.Init(context.Background(), connectionDetails, true)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !db.Initialized {
		t.Fatal("Database should be initalized")
	}

	err = db.Close()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Test decoding a string value for max_open_connections
	connectionDetails = map[string]interface{}{
		"connection_url":       connURL,
		"max_open_connections": "5",
	}

	_, err = db.Init(context.Background(), connectionDetails, true)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestSYBASE_CreateUser(t *testing.T) {
	if os.Getenv("SYBASE_URL") == "" || os.Getenv("VAULT_ACC") != "1" {
		return
	}
	connURL := os.Getenv("SYBASE_URL")

	connectionDetails := map[string]interface{}{
		"connection_url": connURL,
	}

	db := new()
	_, err := db.Init(context.Background(), connectionDetails, true)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	usernameConfig := dbplugin.UsernameConfig{
		DisplayName: "test",
		RoleName:    "test",
	}

	// Test with no configured Creation Statement
	_, _, err = db.CreateUser(context.Background(), dbplugin.Statements{}, usernameConfig, time.Now().Add(time.Minute))
	if err == nil {
		t.Fatal("Expected error when no creation statement is provided")
	}

	statements := dbplugin.Statements{
		Creation: []string{testSYBASERole},
	}

	username, password, err := db.CreateUser(context.Background(), statements, usernameConfig, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if err = testCredsExist(t, connURL, username, password); err != nil {
		t.Fatalf("Could not connect with new credentials: %s", err)
	}
}

func TestSYBASE_RotateRootCredentials(t *testing.T) {
	if os.Getenv("SYBASE_URL") == "" || os.Getenv("VAULT_ACC") != "1" {
		return
	}
	connURL := os.Getenv("SYBASE_URL")
	connectionDetails := map[string]interface{}{
		"connection_url": connURL,
		"username":       "sa",
		"password":       "Sybase123",
	}

	db := new()

	connProducer := db.SQLConnectionProducer

	_, err := db.Init(context.Background(), connectionDetails, true)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !connProducer.Initialized {
		t.Fatal("Database should be initalized")
	}

	newConf, err := db.RotateRootCredentials(context.Background(), nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if newConf["password"] == "yourStrong(!)Password" {
		t.Fatal("password was not updated")
	}

	err = db.Close()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestSYBASE_RevokeUser(t *testing.T) {
	if os.Getenv("SYBASE_URL") == "" || os.Getenv("VAULT_ACC") != "1" {
		return
	}
	connURL := os.Getenv("SYBASE_URL")

	connectionDetails := map[string]interface{}{
		"connection_url": connURL,
	}

	db := new()
	_, err := db.Init(context.Background(), connectionDetails, true)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	statements := dbplugin.Statements{
		Creation: []string{testSYBASERole},
	}

	usernameConfig := dbplugin.UsernameConfig{
		DisplayName: "test",
		RoleName:    "test",
	}

	username, password, err := db.CreateUser(context.Background(), statements, usernameConfig, time.Now().Add(2*time.Second))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if err = testCredsExist(t, connURL, username, password); err != nil {
		t.Fatalf("Could not connect with new credentials: %s", err)
	}

	// Test default revoke statements
	err = db.RevokeUser(context.Background(), statements, username)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if err := testCredsExist(t, connURL, username, password); err == nil {
		t.Fatal("Credentials were not revoked")
	}

	username, password, err = db.CreateUser(context.Background(), statements, usernameConfig, time.Now().Add(2*time.Second))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if err = testCredsExist(t, connURL, username, password); err != nil {
		t.Fatalf("Could not connect with new credentials: %s", err)
	}

	// Test custom revoke statement
	statements.Revocation = []string{testSYBASEDrop}
	err = db.RevokeUser(context.Background(), statements, username)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if err := testCredsExist(t, connURL, username, password); err == nil {
		t.Fatal("Credentials were not revoked")
	}
}

func testCredsExist(t testing.TB, connURL, username, password string) error {
	// Log in with the new creds
	// Expect connURL to be host:port:database
	// database should be "vault"
	parts := strings.Split(connURL, ":")
	connURL = fmt.Sprintf("server=%s;port=%s;user id=%s;password=%s;database=%s;app name=%s;encrypt=false", parts[0], parts[1], username, password, parts[2], parts[2])
	// Note that the gofreetds driver is registered as "mssql"
	// even though it supports Sybase
	db, err := sql.Open("mssql", connURL)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Ping()
}

// We are hard-coding database to "vault" for now
const testSYBASERole = `
CREATE LOGIN {{name}} WITH PASSWORD {{password}} default database vault
USE vault
sp_adduser [{{name}}]
`

const testSYBASEDrop = `
sp_dropuser {{name}}
DROP LOGIN {{name}}
`
