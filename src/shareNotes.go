package main

import (
    "fmt"
    "html/template"
    "net/http"
    "regexp"
    "strconv"
    "database/manager"
    "note"
    "log"
)

type htmlTable struct {
    Notes []note.Note
}

var dbManager = manager.New()

var templates = template.Must(template.ParseFiles("index.html", "AddNote.html", "NoteDetails.html"))

func indexHandler(writer http.ResponseWriter, request *http.Request) {    
    var err error
    var notes []note.Note
    notes, err = dbManager.LoadNotes()
    
    if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
        return
	}
    
    table := htmlTable{Notes : notes }
    
	err = templates.ExecuteTemplate(writer, "index.html", table)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
        return
	}
}

func addNoteHandler(writer http.ResponseWriter, request *http.Request) {
	err := templates.ExecuteTemplate(writer, "AddNote.html", note.Note{})
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
        return
	}
}

func newNoteHandler(writer http.ResponseWriter, request *http.Request) {
    title := request.FormValue("title")
    text := request.FormValue("text")
    
    err := dbManager.AddNote(note.New(title, text))
    
    if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
        return
	}
    
    fmt.Println("####\nAdd Note\n"+title+"\n"+text+"\n####")
    
    http.Redirect(writer, request, "/", http.StatusFound)
}

func noteDetailsHandler(writer http.ResponseWriter, request *http.Request, noteID int) {    
    // Get Note in DB
    fmt.Println("####\nGet Note "+strconv.Itoa(noteID)+" Details\n####")
    
	err := templates.ExecuteTemplate(writer, "NoteDetails.html", note.New("Butt", "Buttocks"))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

func saveNoteHandler(writer http.ResponseWriter, request *http.Request, noteID int) {
    title := request.FormValue("title")
    text := request.FormValue("text")
    // Update Note in DB
    
    fmt.Println("####\nNote "+strconv.Itoa(noteID)+" saved\n"+title+"\n"+text+"\n####")
    
    http.Redirect(writer, request, "/", http.StatusFound)
}

var validPath = regexp.MustCompile("^/(NoteDetails|SaveNote)/([0-9]+)$")

func makeDetailsHandler(function func(http.ResponseWriter, *http.Request, int)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		urlTokens := validPath.FindStringSubmatch(request.URL.Path)
		if urlTokens == nil {
			http.NotFound(writer, request)
			return
		}
        id, _ := strconv.Atoi(urlTokens[2])
		function(writer, request, id)
	}
}

func main() {
    http.HandleFunc("/", indexHandler)
    http.HandleFunc("/AddNote/", addNoteHandler)
    http.HandleFunc("/NewNote/", newNoteHandler)
    http.HandleFunc("/NoteDetails/", makeDetailsHandler(noteDetailsHandler))
    http.HandleFunc("/SaveNote/", makeDetailsHandler(saveNoteHandler))

    err := dbManager.Open()
    
    if err != nil {
		log.Fatal(err)
        return 
	}
    
    defer dbManager.Close()

    log.Printf("ShareNotes initialized...")

	http.ListenAndServe(":8080", nil)
}
