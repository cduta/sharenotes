package main

import (
	"bytes"
	"database/manager"
	"fmt"
	"github.com/mvdan/xurls"
	"html/template"
	"log"
	"net/http"
	"note"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
    "time"
)

type htmlTable struct {
	Notes []htmlNote
    Filtered bool
}

type htmlNote struct {
	NoteID     int
	Title      string
	Text       template.HTML
    AddDate    time.Time
	ChangeDate time.Time
}

func partialHtmlParser(text string) template.HTML {
	var links []string = xurls.Relaxed.FindAllString(text, -1)
	var newText string = ""
	var restText string = text

	if len(links) > 0 {
		currentLink := links[0]

		for len(links) > 0 {
			if strings.HasPrefix(restText, currentLink) {
                if strings.Contains(currentLink, "http://") ||
                   strings.Contains(currentLink, "bitcoin://") ||
                   strings.Contains(currentLink, "file://") ||
                   strings.Contains(currentLink, "magnet://") ||
                   strings.Contains(currentLink, "mailto://") ||
                   strings.Contains(currentLink, "sms://") ||
                   strings.Contains(currentLink, "tel://") ||
                   strings.Contains(currentLink, "smp://") {
                    newText = newText + "<a href=" + currentLink + " target=_new>" + currentLink + "</a>"
                } else {
                    newText = newText + "<a href=http://" + currentLink + " target=_new>" + currentLink + "</a>"
                }
				restText = restText[len(currentLink):len(restText)]
				if len(links) > 1 {
					links = links[1:len(links)]
					currentLink = links[0]
				} else if len(links) == 1 {
					currentLink = links[0]
					links = links[0:0]
				}
			}

			if len(restText) > 1 {
				newText = newText + restText[0:1]
				restText = restText[1:len(restText)]
			} else if len(restText) == 1 {
				newText = newText + restText[0:1]
				restText = restText[0:0]
			}
		}
	} else {
		newText = restText
	}

	var lines []string = strings.Split(newText, "\n")
	return template.HTML("<div>" + strings.Join(lines, "</div>\n<div>") + "</div>")
}

func noteToHtmlNote(note note.Note) htmlNote {
	return htmlNote{
		NoteID:     note.NoteID(),
		Title:      note.Title(),
		Text:       partialHtmlParser(note.Text()),
        AddDate:    note.AddDate(),
        ChangeDate: note.ChangeDate()}
}

var dbManager = manager.New()

var templates = template.Must(template.ParseFiles("index.html", "AddNote.html", "Note.html", "DeleteNote.html", "PasteBinNote.html", "EditNote.html"))

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

	table := htmlTable{Notes: htmlNotes, Filtered: false}

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

func editNoteHandler(writer http.ResponseWriter, request *http.Request, noteID int) {
	var err error
	var foundNote note.Note

	//fmt.Println("####\nGet Note "+strconv.Itoa(noteID)+"\n####")

	foundNote, err = dbManager.GetNote(noteID)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	err = templates.ExecuteTemplate(writer, "EditNote.html", foundNote)
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

	var dirtyBit bool = false
	if foundNote.Title() != title {
		dirtyBit = true
		foundNote.SetTitle(title)
	}

	if foundNote.Text() != text {
		dirtyBit = true
		foundNote.SetText(text)
	}

	if dirtyBit {
		err = dbManager.UpdateNote(foundNote)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(writer, request, fmt.Sprintf("/Note/%d",noteID), http.StatusFound)
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

func confirmPasteBinNoteHandler(writer http.ResponseWriter, request *http.Request, noteID int) {
	var err error
	var foundNote note.Note

	foundNote, err = dbManager.GetNote(noteID)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	err = templates.ExecuteTemplate(writer, "PasteBinNote.html", foundNote)
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
		"-F", fmt.Sprintf("title=\"%s (ID:%d)\"", foundNote.Title(), foundNote.NoteID()),
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

func filteredIndexHandler1(writer http.ResponseWriter, request *http.Request, whereClause string, filterText string) {
	var err error
	var notes []note.Note
	notes, err = dbManager.LoadNotesWhere1(whereClause, filterText)

	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
    
	var htmlNotes []htmlNote

	for _, n := range notes {
		htmlNotes = append(htmlNotes, noteToHtmlNote(n))
	}

	table := htmlTable{Notes: htmlNotes, Filtered: true}

	err = templates.ExecuteTemplate(writer, "index.html", table)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
}

func filteredIndexHandler2(writer http.ResponseWriter, request *http.Request, whereClause string, filterText string) {
	var err error
	var notes []note.Note
	notes, err = dbManager.LoadNotesWhere2(whereClause, filterText, filterText)

	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	var htmlNotes []htmlNote

	for _, n := range notes {
		htmlNotes = append(htmlNotes, noteToHtmlNote(n))
	}

	table := htmlTable{Notes: htmlNotes, Filtered: true}

	err = templates.ExecuteTemplate(writer, "index.html", table)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
}

func titleFilterHandler(writer http.ResponseWriter, request *http.Request, filterInput string) {
    filteredIndexHandler1(writer, request, manager.SELECT_NOTES_WHERE_TITLE_QS, filterInput)
}

func textFilterHandler(writer http.ResponseWriter, request *http.Request, filterInput string) {
    filteredIndexHandler1(writer, request, manager.SELECT_NOTES_WHERE_TEXT_QS, filterInput)
}

func bothFilterHandler(writer http.ResponseWriter, request *http.Request, filterInput string) {
    filteredIndexHandler2(writer, request, manager.SELECT_NOTES_WHERE_BOTH_QS, filterInput)
}

var validPath = regexp.MustCompile("^/(Note|EditNote|SaveNote|ConfirmDeleteNote|DeleteNote|PasteBinNote|ConfirmPasteBinNote)/([0-9]+)$")

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

var validFilterPath = regexp.MustCompile("^/(Title|Text|Both)Filter/([0-9a-zA-Z]+)$")

func makeFilterHandler(function func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		urlTokens := validFilterPath.FindStringSubmatch(request.URL.Path)
		if urlTokens == nil {
			http.NotFound(writer, request)
			return
		}
		function(writer, request, urlTokens[2])
	}
}

func main() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/AddNote/", addNoteHandler)
	http.HandleFunc("/NewNote/", newNoteHandler)
	http.HandleFunc("/Note/", makeNoteIDHandler(noteDetailsHandler))
    http.HandleFunc("/EditNote/", makeNoteIDHandler(editNoteHandler))
	http.HandleFunc("/SaveNote/", makeNoteIDHandler(saveNoteHandler))
	http.HandleFunc("/ConfirmDeleteNote/", makeNoteIDHandler(confirmDeleteNoteHandler))
	http.HandleFunc("/DeleteNote/", makeNoteIDHandler(deleteNoteHandler))
	http.HandleFunc("/ConfirmPasteBinNote/", makeNoteIDHandler(confirmPasteBinNoteHandler))
	http.HandleFunc("/PasteBinNote/", makeNoteIDHandler(pasteBinNoteHandler))
    http.HandleFunc("/TitleFilter/", makeFilterHandler(titleFilterHandler))
    http.HandleFunc("/TextFilter/", makeFilterHandler(textFilterHandler))
    http.HandleFunc("/BothFilter/", makeFilterHandler(bothFilterHandler))

	err := dbManager.Open()

	if err != nil {
		log.Fatal(err)
		return
	}

	defer dbManager.Close()

	log.Printf("ShareNotes initialized...")

	http.ListenAndServe(":8080", nil)
}
