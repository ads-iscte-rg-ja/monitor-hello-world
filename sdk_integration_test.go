package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	monitorsdk "github.com/ads-iscte-rg-ja/monitor-go-sdk"
)

// Test fixtures only: emulate the database remote prepares using its actual
// local SQL file. Application production code never executes this schema.
func preparedTestBuffer(t *testing.T) (*sql.DB, string) {
	t.Helper()
	schema, err := os.ReadFile(filepath.Join("..", "monitor-remote", "database", "schema.sql"))
	if err != nil {
		t.Fatal("this integration test requires the workspace's local remote schema", err)
	}
	path := filepath.Join(t.TempDir(), "prepared.db")
	u := url.URL{Scheme: "file", Path: path}
	q := u.Query()
	q.Set("_journal_mode", "WAL")
	q.Set("_synchronous", "FULL")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite3", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	return db, path
}

func TestSDKWritesPreparedBufferAtomically(t *testing.T) {
	fixture, path := preparedTestBuffer(t)
	application, err := monitorsdk.ConnectDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	stack := "diagnostic stack"
	id, err := monitorsdk.LogError(context.Background(), application, "app", "test", "ERROR", "failure", monitorsdk.ErrorDetails{StackTrace: &stack, Metadata: json.RawMessage(`{"route":"/orders"}`)})
	if err != nil || id <= 0 {
		t.Fatal(id, err)
	}
	var queuedID int64
	var storedStack string
	if err := fixture.QueryRow(`SELECT e.id,e.stack_trace FROM error_logs e JOIN delivery_status d ON d.record_type='error_log' AND d.record_id=e.id WHERE d.sent_at IS NULL`).Scan(&queuedID, &storedStack); err != nil || queuedID != id || storedStack != stack {
		t.Fatal("shared SQLite contract mismatch", err)
	}
	if _, err := fixture.Exec(`CREATE TRIGGER fail_queue BEFORE INSERT ON delivery_status BEGIN SELECT RAISE(ABORT,'injected queue failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := monitorsdk.LogError(context.Background(), application, "app", "test", "ERROR", "must roll back"); err == nil {
		t.Fatal("queue failure was ignored")
	}
	var count int
	if err := fixture.QueryRow("SELECT COUNT(*) FROM error_logs").Scan(&count); err != nil || count != 1 {
		t.Fatal("error escaped queue transaction", err)
	}
}

func TestApplicationUsesSDKWithoutInitializingTables(t *testing.T) {
	fixture, path := preparedTestBuffer(t)
	if err := run(context.Background(), "logs", path, 1); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), "panic", path, 1); err != nil {
		t.Fatal(err)
	}
	var logs, pending int
	if err := fixture.QueryRow("SELECT COUNT(*) FROM error_logs").Scan(&logs); err != nil {
		t.Fatal(err)
	}
	if err := fixture.QueryRow("SELECT COUNT(*) FROM delivery_status WHERE sent_at IS NULL").Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if logs != 6 || pending != 6 {
		t.Fatalf("logs=%d pending=%d, want 6 each", logs, pending)
	}
}

func TestSDKOptionsSurviveReplacementConnection(t *testing.T) {
	_, path := preparedTestBuffer(t)
	db, err := monitorsdk.ConnectDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxIdleConns(0)
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, setting := range []struct {
		name string
		want int
	}{{"busy_timeout", 5000}, {"foreign_keys", 1}, {"synchronous", 2}} {
		var value int
		if err := conn.QueryRowContext(context.Background(), "PRAGMA "+setting.name).Scan(&value); err != nil || value != setting.want {
			t.Fatalf("%s=%d want=%d err=%v", setting.name, value, setting.want, err)
		}
	}
}
