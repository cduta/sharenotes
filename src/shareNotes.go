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

var templates = template.Must(template.ParseFiles("index.html", "AddNote.html", "Note.html"))

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
    
    //fmt.Println("####\nAdd Note\n"+title+"\n"+text+"\n####")
    
    http.Redirect(writer, request, "/", http.StatusFound)
}

func noteDetailsHandler(writer http.ResponseWriter, request *http.Request, noteID int) {    
    var err error
    var foundNote note.Note
    
    //fmt.Println("####\nGet Note "+strconv.Itoa(noteID)+"\n####")

    foundNote, err = dbManager.GetNote(noteID)
    if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
        return
	}
        
	err = templates.ExecuteTemplate(writer, "Note.html", foundNote)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
        return
	}
}

func saveNoteHandler(writer http.ResponseWriter, request *http.Request, noteID int) {
    foundNote, err := dbManager.GetNote(noteID)
    if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
        return
	}
    
    title := request.FormValue("title")
    text := request.FormValue("text")
    
    //fmt.Println("####\nNote "+strconv.Itoa(noteID)+" saved\n"+title+"\n"+text+"\n####")
    
    var dirtyBit bool = false;
    if(foundNote.Title() != title) {
        dirtyBit = true;
        foundNote.SetTitle(title)
    }
    
    if(foundNote.Text() != text) {
        dirtyBit = true;
        foundNote.SetText(text);
    }
    
    if(dirtyBit) {
        dbManager.UpdateNote(foundNote)
    }
    
    http.Redirect(writer, request, "/", http.StatusFound)
}

var validPath = regexp.MustCompile("^/(Note|SaveNote)/([0-9]+)$")

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
    http.HandleFunc("/Note/", makeDetailsHandler(noteDetailsHandler))
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
