package manager

import (
    "database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
    "log"
	"os"
    "note"
    "time"
)

const DB_FILE_NAME = "sndb.db"

const INITIALIZE_NOTES_TABLE_EXEC = 
    `create table notes (
        noteID integer not null primary key, 
        title text, 
        text text, 
        addDate DATETIME, 
        changeDate DATETIME
    );`
    
const SELECT_NOTES_QS = 
    `select noteID, title, text, addDate, changeDate 
     from notes`
     
const ADD_NOTES_QS = 
    `insert into notes(title, text, addDate, changeDate) 
     values(?, ?, ?, ?)`

type DatabaseManager struct {
    notes []note.Note
    db *sql.DB
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
            fmt.Println(noteID, title, text, addDate, changeDate)
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
    
    if(dbm.checkDBValidity() != nil) {
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

func toSQLiteDateTimeString(t time.Time) string {
    return fmt.Sprintf("", t.Year(), "-", t.Month(), "-", t.Day(), " ", t.Hour(), ":", t.Minute(), ":", t.Second())
}

func (dbm *DatabaseManager) AddNote(n note.Note) error {
    
    transaction, err := dbm.db.Begin()
	if err != nil {
        log.Printf("%q: %s\n", err, "Beginnin add transaction.")
        return err
	}
	
    stmt, err := transaction.Prepare(ADD_NOTES_QS)
	if err != nil {
        log.Printf("%q: %s\n", err, "Preparing add transaction.")
        return err
	}
	defer stmt.Close()
	
    _, err = stmt.Exec(n.Title(), n.Text(), toSQLiteDateTimeString(n.AddDate()), toSQLiteDateTimeString(n.ChangeDate()))
    if err != nil {
        log.Printf("%q: %s\n", err, "Add note in add transaction.")
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
            var addDate time.Time
            var changeDate time.Time
            rows.Scan(&noteID, &title, &text, &addDate, &changeDate)
            dbm.notes = append(dbm.notes, note.NewLocal(noteID, title, text, addDate, changeDate))
        }
    }

    return dbm.notes, err
}
