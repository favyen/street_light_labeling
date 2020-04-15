package main

import (
	_ "github.com/mattn/go-sqlite3"

	"database/sql"
)

var db *Database

type Database struct {
	db *sql.DB
}

func init() {
	sdb, err := sql.Open("sqlite3", "./app.sqlite3")
	if err != nil {
		panic(err)
	}
	db = &Database{db: sdb}

	db.Exec(`CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY ASC,
		fname TEXT
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS orig_points (
		id INTEGER PRIMARY KEY ASC,
		fname TEXT,
		x INTEGER,
		y INTEGER
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS label_points (
		id INTEGER PRIMARY KEY ASC,
		fname TEXT,
		x INTEGER,
		y INTEGER
	)`)
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func (this *Database) Query(q string, args ...interface{}) Rows {
	rows, err := this.db.Query(q, args...)
	checkErr(err)
	return Rows{rows}
}

func (this *Database) QueryRow(q string, args ...interface{}) Row {
	row := this.db.QueryRow(q, args...)
	return Row{row}
}

func (this *Database) Exec(q string, args ...interface{}) Result {
	result, err := this.db.Exec(q, args...)
	checkErr(err)
	return Result{result}
}

type Rows struct {
	rows *sql.Rows
}

func (r Rows) Close() {
	err := r.rows.Close()
	checkErr(err)
}

func (r Rows) Next() bool {
	return r.rows.Next()
}

func (r Rows) Scan(dest ...interface{}) {
	err := r.rows.Scan(dest...)
	checkErr(err)
}

type Row struct {
	row *sql.Row
}

func (r Row) Scan(dest ...interface{}) {
	err := r.row.Scan(dest...)
	checkErr(err)
}

type Result struct {
	result sql.Result
}

func (r Result) LastInsertId() int {
	id, err := r.result.LastInsertId()
	checkErr(err)
	return int(id)
}

func (r Result) RowsAffected() int {
	count, err := r.result.RowsAffected()
	checkErr(err)
	return int(count)
}
