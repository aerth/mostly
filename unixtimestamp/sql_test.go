package unixtimestamp_test

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/aerth/mostly/unixtimestamp"
	// _ "github.com/mattn/go-sqlite3" uncomment to run this test
)

type UnixTimestamp = unixtimestamp.UnixTimestampNull
type UnixTimestampNotNull = unixtimestamp.UnixTimestampNotNull

var Now = unixtimestamp.Now

func TestSql(t *testing.T) {
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared")
	if err != nil && strings.Contains(err.Error(), "forgotten import") {
		log.Printf("%T %v", err, err)
		println("skipping unixtimestamp.TestSql due to no sqlite3 driver (uncomment import)")
		t.SkipNow()
		return
	}
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	var (
		t1    *UnixTimestamp = Now()
		y     *UnixTimestamp = nil
		empty *UnixTimestamp = nil
	)
	{
		_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, t TIMESTAMP NULL DEFAULT NULL);")
		if err != nil {
			t.Fatalf("CREATE TABLE: %v", err)
		}
		_, err = db.Exec("INSERT INTO test (t) VALUES (?)", t1)
		if err != nil {
			t.Fatalf("INSERT: %v", err)
		}
		err = db.QueryRow("SELECT t FROM test").Scan(&y)
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
		if y == nil {
			t.Fatalf("Scan: nil")
		}
		println("parsed:  ", y.String())
		if y.After(t1.Add(time.Second)) {
			t.Fatalf("After")
		}
		if y.Before(t1.Add(-time.Second)) {
			t.Fatalf("Before")
		}
	}
	{
		// whats in the db? (not null...)
		var z string
		err = db.QueryRow("SELECT t FROM test").Scan(&z)
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
		println("raw:     ", z)
		if z == "" {
			t.Fatalf("should not be empty")
		}
		if _, err = time.Parse(time.RFC3339Nano, z); err != nil {
			t.Fatalf("Parse: %v", err)
		}

	}

	{
		// whats a null in the db?
		var z *UnixTimestamp
		_, err = db.Exec("DELETE FROM test;")
		if err != nil {
			t.Fatalf("INSERT: %v", err)
		}
		_, err = db.Exec("INSERT INTO test (t) VALUES (?)", empty)
		if err != nil {
			t.Fatalf("INSERT: %v", err)
		}
		err = db.QueryRow("SELECT t FROM test").Scan(&z)
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
		if z != nil {
			t.Fatalf("should be empty: %v", z)
		}
		println("null:    ", fmt.Sprint(z))
	}

	{
		// whats an empty NotNull in the db?
		_, err = db.Exec("DROP TABLE test; CREATE TABLE test (id INTEGER PRIMARY KEY, t TIMESTAMP)")
		if err != nil {
			t.Fatalf("INSERT: %v", err)
		}
		_, err = db.Exec("INSERT INTO test (t) VALUES (?)", (*UnixTimestampNotNull)(nil))
		if err != nil {
			t.Fatalf("INSERT NOTNULL: %v", err)
		}

		// use rawbytes to get the raw value (?)
		var raw sql.RawBytes
		rows, err := db.Query("SELECT t FROM test")
		if err != nil {
			t.Fatalf("SELECT: %v", err)
		}
		defer rows.Close()
		for rows.Next() {
			err = rows.Scan(&raw)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			if len(raw) == 0 {
				t.Fatalf("should not be empty: %v", raw)
			}
			println("notnullraw:", fmt.Sprint(string(raw)))
			if parsed, err := time.Parse(time.RFC3339Nano, string(raw)); err != nil {
				t.Fatalf("Parse: %v", err)
			} else {
				println("notnull:   ", parsed.String())
			}
		}
	}
}
