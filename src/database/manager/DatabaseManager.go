package manager

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"note"
	"os"
	"strconv"
	"time"
)

const DB_FILE_NAME = "sndb.db"

const DATE_FORMAT = "1999-12-31 24:12:59"

const INITIALIZE_NOTES_TABLE_EXEC = `create table notes (
        noteID integer not null primary key, 
        title text, 
        text text, 
        addDate time, 
        changeDate time
    );`

const SELECT_NOTES_QS = `select noteID, title, text, addDate, changeDate 
     from notes
     order by changeDate desc`

const LOOKUP_NOTE_QS = `select title, text, addDate, changeDate 
     from notes
     where noteID = ?`

const SELECT_NOTES_WHERE_TITLE_QS = `select noteID, title, text, addDate, changeDate 
     from notes
     where title like ?
     order by changeDate desc`

const SELECT_NOTES_WHERE_TEXT_QS = `select noteID, title, text, addDate, changeDate 
     from notes
     where text like ?
     order by changeDate desc`

const SELECT_NOTES_WHERE_BOTH_QS = `select noteID, title, text, addDate, changeDate 
     from notes
     where title like ? or text like ?
     order by changeDate desc`

const ADD_NOTE_EXEC = `insert into notes(title, text, addDate, changeDate) 
     values(?, ?, ?, ?);`

const UPDATE_NOTE_EXEC = `update notes 
     set title = ?, text = ?, changeDate = ?
     where noteID = ?;`

const DELETE_NOTE_EXEC = `delete from notes 
     where noteID = ?;`

type DatabaseManager struct {
	notes []note.Note
	db    *sql.DB
}

func New() DatabaseManager {
	dbm := DatabaseManager{}

	return dbm
}

func (dbm *DatabaseManager) Notes() []note.Note {
	return dbm.notes
}

func (dbm *DatabaseManager) checkDBValidity() error {
	rows, err := dbm.db.Query(SELECT_NOTES_QS)

	if err != nil {
		log.Printf("%q: %s\n", err, "Rebuilding the database...")
	} else {
		defer rows.Close()
		for rows.Next() {
			var noteID int
			var title string
			var text string
			var addDate time.Time
			var changeDate time.Time
			rows.Scan(&noteID, &title, &text, &addDate, &changeDate)
		}
	}

	return err
}

func (dbm *DatabaseManager) initializeDB() error {
	var err error

	os.Remove("./foo.db")

	dbm.db, err = sql.Open("sqlite3", "./"+DB_FILE_NAME)

	_, err = dbm.db.Exec(INITIALIZE_NOTES_TABLE_EXEC)

	if err != nil {
		log.Printf("%q: %s\n", err, INITIALIZE_NOTES_TABLE_EXEC)
	}

	return err
}

func (dbm *DatabaseManager) Open() error {
	var err error

	dbm.db, err = sql.Open("sqlite3", "./"+DB_FILE_NAME)

	if err != nil {
		log.Fatal(err)
		return err
	}

	if dbm.checkDBValidity() != nil {
		dbm.Close()
		if err != nil {
			log.Fatal(err)
		} else {
			dbm.initializeDB()
		}
	}

	return err
}

func (dbm *DatabaseManager) Close() {
	dbm.db.Close()
}

func (dbm *DatabaseManager) AddNote(n note.Note) error {

	transaction, err := dbm.db.Begin()
	if err != nil {
		log.Printf("%q: %s\n", err, "Initializing add transaction.")
		return err
	}

	stmt, err := transaction.Prepare(ADD_NOTE_EXEC)
	if err != nil {
		log.Printf("%q: %s\n", err, "Preparing add transaction.")
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(n.Title(), n.Text(), n.AddDate().Unix(), n.ChangeDate().Unix())
	if err != nil {
		log.Printf("%q: %s\n", err, "Add note in add transaction.")
		return err
	}

	transaction.Commit()

	return err
}

func (dbm *DatabaseManager) UpdateNote(n note.Note) error {
	transaction, err := dbm.db.Begin()
	if err != nil {
		log.Printf("%q: %s\n", err, "Initializing update transaction.")
		return err
	}

	updateStatement, err := transaction.Prepare(UPDATE_NOTE_EXEC)
	if err != nil {
		log.Printf("%q: %s\n", err, "Preparing update transaction.")
		return err
	}
	defer updateStatement.Close()

	_, err = updateStatement.Exec(n.Title(), n.Text(), n.ChangeDate().Unix(), strconv.Itoa(n.NoteID()))
	if err != nil {
		log.Printf("%q: %s\n", err, "Update note in update transaction.")
		return err
	}

	transaction.Commit()

	return err
}

func (dbm *DatabaseManager) DeleteNote(noteID int) error {
	transaction, err := dbm.db.Begin()
	if err != nil {
		log.Printf("%q: %s\n", err, "Initializing delete transaction.")
		return err
	}

	deleteStatement, err := transaction.Prepare(DELETE_NOTE_EXEC)
	if err != nil {
		log.Printf("%q: %s\n", err, "Preparing delete transaction.")
		return err
	}
	defer deleteStatement.Close()

	_, err = deleteStatement.Exec(strconv.Itoa(noteID))
	if err != nil {
		log.Printf("%q: %s\n", err, "Update note in delete transaction.")
		return err
	}

	transaction.Commit()

	return err
}

func (dbm *DatabaseManager) LoadNotes() ([]note.Note, error) {
	dbm.notes = dbm.notes[0:0]

	rows, err := dbm.db.Query(SELECT_NOTES_QS)

	if err != nil {
		log.Printf("%q: %s\n", err, SELECT_NOTES_QS)
	} else {
		defer rows.Close()
		for rows.Next() {
			var noteID int
			var title string
			var text string
			var addDate int64
			var changeDate int64
			rows.Scan(&noteID, &title, &text, &addDate, &changeDate)
			dbm.notes = append(dbm.notes, note.NewLocal(noteID, title, text, time.Unix(addDate, 0), time.Unix(changeDate, 0)))
		}
	}

	return dbm.notes, err
}

func (dbm *DatabaseManager) LoadNotesWhere(whereClause string, whereParameters ...string) ([]note.Note, error) {
	dbm.notes = dbm.notes[0:0]

	whereQuery, err := dbm.db.Prepare(whereClause)
	if err != nil {
		log.Printf("%q: %s\n", err, "Initializing select notes where transaction.")
		return []note.Note{}, err
	}

	defer whereQuery.Close()

        wp := make([]interface{}, len(whereParameters))
        for i, ps := range whereParameters {
            wp[i] = ps
        }

	rows, err := whereQuery.Query(wp...)

	if err != nil {
		log.Printf("%q: %s\n", err, "Query select notes where transaction.")
	} else {
		defer rows.Close()
		for rows.Next() {
			var noteID int
			var title string
			var text string
			var addDate int64
			var changeDate int64
			rows.Scan(&noteID, &title, &text, &addDate, &changeDate)
			dbm.notes = append(dbm.notes, note.NewLocal(noteID, title, text, time.Unix(addDate, 0), time.Unix(changeDate, 0)))
		}
	}

	return dbm.notes, err
}

func (dbm *DatabaseManager) GetNote(noteID int) (note.Note, error) {

	lookupQuery, err := dbm.db.Prepare(LOOKUP_NOTE_QS)
	if err != nil {
		log.Printf("%q: %s\n", err, LOOKUP_NOTE_QS)
		return note.Note{}, err
	}

	defer lookupQuery.Close()

	var title string
	var text string
	var addDate int64
	var changeDate int64

	err = lookupQuery.QueryRow(strconv.Itoa(noteID)).Scan(&title, &text, &addDate, &changeDate)
	if err != nil {
		log.Printf("%q: %s\n", err, "Get Note scan failed.")
		return note.Note{}, err
	}

	return note.NewLocal(noteID, title, text, time.Unix(addDate, 0), time.Unix(changeDate, 0)), err
}
