package database

import (
	"database/sql"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Event struct {
	Id int;
	Start time.Time;
	End time.Time;
	Location string;
	Summary string;
	Description string;
}

type Database struct {
	db *sql.DB
}

func NewDatabase() (*Database, error) {
	if err := os.MkdirAll("./db", 0755); err != nil {
		return nil, err
	}
	
	db, err := sql.Open("sqlite3", "./db/database.db")
	if err != nil {
		return nil, err
	}
	database := &Database{db}
	err = database.setupDatabase()
	if err != nil {
		return nil, err
	}
	return database, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) setupDatabase() error {
	_, err := d.db.Exec(`
	CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY,
		start TEXT,
		end TEXT,
		location TEXT,
		summary TEXT,
		description TEXT
	)`)
	return err
}

func (d *Database) InsertEvents(event []Event) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO events(id, start, end, location, summary, description) VALUES(?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	for _, e := range event {
		_, err = stmt.Exec(e.Id, e.Start.String(), e.End.String(), e.Location, e.Summary, e.Description)
		if err != nil {
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func (d *Database) GetEvents() ([]Event, error) {
	rows, err := d.db.Query("SELECT id, start, end, location, summary, description FROM events")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []Event
	startString := ""
	endString := ""
	for rows.Next() {
		var e Event
		err = rows.Scan(&e.Id, &startString, &endString, &e.Location, &e.Summary, &e.Description)
		if err != nil {
			return nil, err
		}
		e.Start, _ = time.Parse(time.RFC3339, startString)
		e.End, _ = time.Parse(time.RFC3339, endString)
		events = append(events, e)
	}
	return events, nil
}

func (d *Database) DeleteEvents() error {
	_, err := d.db.Exec("DELETE FROM events")
	return err
}
