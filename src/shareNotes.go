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
    "strings"
    "github.com/mvdan/xurls"
    "sort"
)

type htmlTable struct {
    Notes []htmlNote
}

type htmlNote struct {
    NoteID int
    Title string
    Text template.HTML
}

func partialHtmlParser(text string) template.HTML {
    var links []string = xurls.Relaxed.FindAllString(text, -1)
    sort.Strings(links)
    
    printf()
    
    var lines []string = strings.Split(text, "\n")
    
    return template.HTML("<div>" + strings.Join(lines, "</div>\n<div>") + "</div>")
}

func noteToHtmlNote(note note.Note) htmlNote {
    return htmlNote{
        NoteID: note.NoteID(),
        Title: note.Title(),
        Text: partialHtmlParser(note.Text())}
}

var dbManager = manager.New()

var templates = template.Must(template.ParseFiles("index.html", "AddNote.html", "Note.html", "DeleteNote.html", "PasteBinNote.html"))

func indexHandler(writer http.ResponseWriter, request *http.Request) {    
    var err error
    var notes []note.Note
    notes, err = dbManager.LoadNotes()
    
    if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
        return
	}
    
    var htmlNotes []htmlNote
    
    for _, n := range notes {
        htmlNotes = append(htmlNotes, noteToHtmlNote(n))
    }
    
    table := htmlTable{Notes : htmlNotes }
    
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
        
	err = templates.ExecuteTemplate(writer, "Note.html", noteToHtmlNote(foundNote))
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
        
	err = templates.ExecuteTemplate(writer, "DeleteNote.html", noteToHtmlNote(foundNote))
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

func confirmPasteBinNoteHandler(writer http.ResponseWriter, request *http.Request, noteID int) {
    var err error
    var foundNote note.Note

    foundNote, err = dbManager.GetNote(noteID)
    if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
        return
	}
        
	err = templates.ExecuteTemplate(writer, "PasteBinNote.html", noteToHtmlNote(foundNote))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
        return
	}
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

var validPath = regexp.MustCompile("^/(Note|SaveNote|ConfirmDeleteNote|DeleteNote|PasteBinNote|ConfirmPasteBinNote)/([0-9]+)$")

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
    http.HandleFunc("/ConfirmPasteBinNote/", makeNoteIDHandler(confirmPasteBinNoteHandler))
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
