package note

import (
        "time"
)

type Note struct {
    noteID int
    title string
    text string
    addDate time.Time
    changeDate time.Time
}

func New(title string, text string) Note {
    n := Note{noteID: 0, title: title, text: text, addDate: time.Now(), changeDate: time.Now()}
    return n
}

func NewLocal(noteID int, title string, text string, addDate time.Time, changeDate time.Time) Note {
    n := Note{noteID: noteID, title: title, text: text, addDate: addDate, changeDate: changeDate}
    return n
}

func (n *Note) SetTitle(title string) {
    n.title = title;
    n.changeDate = time.Now();
}

func (n *Note) SetText(text string) {
    n.text = text;
    n.changeDate = time.Now();
}

func (n Note) NoteID() int {
    return n.noteID
}

func (n Note) Title() string {
    return n.title
}

func (n Note) Text() string {
    return n.text
}

func (n Note) AddDate() time.Time {
    return n.addDate
}

func (n Note) ChangeDate() time.Time {
    return n.changeDate
}
