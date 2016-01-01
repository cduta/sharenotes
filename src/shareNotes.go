package main

import (
    "os/exec"
    "fmt"
    "html/template"
    "net/http"
    "regexp"
    "strconv"
    "database/manager"
    "note"
    "log"
    "bytes"
)

type htmlTable struct {
    Notes []note.Note
}

var dbManager = manager.New()

var templates = template.Must(template.ParseFiles("index.html", "AddNote.html", "Note.html", "DeleteNote.html"))

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
        err = dbManager.UpdateNote(foundNote)
        if err != nil {
            http.Error(writer, err.Error(), http.StatusInternalServerError)
            return
        }
    }
    
    http.Redirect(writer, request, "/", http.StatusFound)
}

func confirmDeleteNoteHandler(writer http.ResponseWriter, request *http.Request, noteID int) {
    var err error
    var foundNote note.Note

    foundNote, err = dbManager.GetNote(noteID)
    if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
        return
	}
        
	err = templates.ExecuteTemplate(writer, "DeleteNote.html", foundNote)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
        return
	}
}

func deleteNoteHandler(writer http.ResponseWriter, request *http.Request, noteID int) {
    err := dbManager.DeleteNote(noteID)
    if err != nil {
        http.Error(writer, err.Error(), http.StatusInternalServerError)
        return
    }
    
    http.Redirect(writer, request, "/", http.StatusFound)
}

func pasteBinNoteHandler(writer http.ResponseWriter, request *http.Request, noteID int) {
    var err error
    var foundNote note.Note

    foundNote, err = dbManager.GetNote(noteID)
    if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
        return
	}
    
    shellCommand := exec.Command("curl", 
        "-s", 
        "-F", fmt.Sprintf("content=%s", foundNote.Text()), 
        "-F", fmt.Sprintf("title=\"%s (ID:%d)\"",foundNote.Title(),foundNote.NoteID()), 
        "http://dpaste.com/api/v2/")
    
	var output bytes.Buffer
	shellCommand.Stdout = &output
	err = shellCommand.Run()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
        return
	}
    
    fmt.Println(output.String())
    
    http.Redirect(writer, request, output.String(), http.StatusFound)
}

var validPath = regexp.MustCompile("^/(Note|SaveNote|ConfirmDeleteNote|DeleteNote|PasteBinNote)/([0-9]+)$")

func makeNoteIDHandler(function func(http.ResponseWriter, *http.Request, int)) http.HandlerFunc {
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
    http.HandleFunc("/Note/", makeNoteIDHandler(noteDetailsHandler))
    http.HandleFunc("/SaveNote/", makeNoteIDHandler(saveNoteHandler))
    http.HandleFunc("/ConfirmDeleteNote/", makeNoteIDHandler(confirmDeleteNoteHandler))
    http.HandleFunc("/DeleteNote/", makeNoteIDHandler(deleteNoteHandler))
    http.HandleFunc("/PasteBinNote/", makeNoteIDHandler(pasteBinNoteHandler))

    err := dbManager.Open()
    
    if err != nil {
        log.Fatal(err)
        return 
	}
    
    defer dbManager.Close()

    log.Printf("ShareNotes initialized...")

	http.ListenAndServe(":8080", nil)
}
