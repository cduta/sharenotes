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
        "math/rand"
        "math"
)

const TOO_MANY_REQUEST_TIME_SPAN_IN_NS = int64(1000)

type synchronizedToken struct {
        ID          uint64
        TokenString string
}

type sessionIDManager struct {
        currentID     uint64
        sessionTokens map[uint64]string
}

var sidManager sessionIDManager = sessionIDManager{ currentID: 0, sessionTokens: make(map[uint64]string) }

const CHARACTERS = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const CHARACTER_INDEX_BITS = 6                    
const CHARACTER_INDEX_MASK = 1 << CHARACTER_INDEX_BITS - 1 
const CHARACTER_INDEX_MAX  = 63 / CHARACTER_INDEX_BITS   

var src = rand.NewSource(time.Now().UnixNano())

func randomStringLength(n int) string {
    var result []byte = make([]byte, n)
    var cache int64 = src.Int63()
    var remain int = CHARACTER_INDEX_MAX
    
    for i := n-1; i >= 0; {
        if remain == 0 {
            cache = src.Int63()
            remain = CHARACTER_INDEX_MAX
        }
        if j := int(cache & CHARACTER_INDEX_MASK); j < len(CHARACTERS) {
            result[i] = CHARACTERS[j]
            i--
        }
        cache >>= CHARACTER_INDEX_BITS
        remain--
    }

    return string(result)
}

func (sidm *sessionIDManager) generateSynchronizedToken() synchronizedToken {
        var sessionToken synchronizedToken = synchronizedToken{ ID: sidm.currentID, TokenString: randomStringLength(100) }
        
        sidm.sessionTokens[sessionToken.ID] = sessionToken.TokenString
        sidm.currentID++
        
        return sessionToken
}

func (sidm *sessionIDManager) synchronizedTokenIsValid(token synchronizedToken) bool {
        return sidm.sessionTokens[token.ID] == token.TokenString;
}

type htmlTable struct {
	Notes    []htmlNote
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

	//var lines []string = strings.Split(newText, "\n")
	//return template.HTML("<div>" + strings.Join(lines, "</div>\n<div>") + "</div>")
        return template.HTML(newText)
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

type addNoteData struct {
        Token synchronizedToken
}

func addNoteHandler(writer http.ResponseWriter, request *http.Request) {
	err := templates.ExecuteTemplate(writer, "AddNote.html", addNoteData{Token: sidManager.generateSynchronizedToken()})
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
}

func newNoteHandler(writer http.ResponseWriter, request *http.Request) {
	title := request.FormValue("title")
	text := request.FormValue("text")
        tokenID := request.FormValue("share_note_token_id")
        tokenString := request.FormValue("share_note_token_string")
        
        id, err := strconv.ParseUint(tokenID, 10, 64)
        
        if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

        if !sidManager.synchronizedTokenIsValid(synchronizedToken{ID: id, TokenString: tokenString}) {
                http.Error(writer, "Token was invlaid.", http.StatusInternalServerError)
                return
        }

	err = dbManager.AddNote(note.New(title, text))

	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

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

type confirmNoteData struct {
        Note note.Note
        Token synchronizedToken
}

func preparePostHandler(writer http.ResponseWriter, request *http.Request, urlName string, noteID int) {
	var err error
	var foundNote note.Note

	foundNote, err = dbManager.GetNote(noteID)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	err = templates.ExecuteTemplate(writer, urlName + ".html", confirmNoteData{Note: foundNote, Token: sidManager.generateSynchronizedToken()})
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
        tokenID := request.FormValue("share_note_token_id")
        tokenString := request.FormValue("share_note_token_string")
        
        id, err := strconv.ParseUint(tokenID, 10, 64)
        
        if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

        if !sidManager.synchronizedTokenIsValid(synchronizedToken{ID: id, TokenString: tokenString}) {
                http.Error(writer, "Token was invlaid.", http.StatusInternalServerError)
                return
        }

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

	http.Redirect(writer, request, fmt.Sprintf("/Note/%d", noteID), http.StatusFound)
}

func deleteNoteHandler(writer http.ResponseWriter, request *http.Request, noteID int) {
	tokenID := request.FormValue("share_note_token_id")
        tokenString := request.FormValue("share_note_token_string")
        
        id, err := strconv.ParseUint(tokenID, 10, 64)
        
        if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

        if !sidManager.synchronizedTokenIsValid(synchronizedToken{ID: id, TokenString: tokenString}) {
                http.Error(writer, "Token was invlaid.", http.StatusInternalServerError)
                return
        }
        
        err = dbManager.DeleteNote(noteID)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(writer, request, "/", http.StatusFound)
}

func pasteBinNoteHandler(writer http.ResponseWriter, request *http.Request, noteID int) {
	var err error
	var foundNote note.Note

        tokenID := request.FormValue("share_note_token_id")
        tokenString := request.FormValue("share_note_token_string")
        
        id, err := strconv.ParseUint(tokenID, 10, 64)
        
        if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

        if !sidManager.synchronizedTokenIsValid(synchronizedToken{ID: id, TokenString: tokenString}) {
                http.Error(writer, "Invalid post request..", http.StatusInternalServerError)
                return
        }

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

        log.Printf(output.String())

	http.Redirect(writer, request, output.String(), http.StatusFound)
}

func filteredIndexHandler(writer http.ResponseWriter, request *http.Request, whereClause string, whereParameters ...string) {
	var err error
	var notes []note.Note
	notes, err = dbManager.LoadNotesWhere(whereClause, whereParameters...)

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
	filteredIndexHandler(writer, request, manager.SELECT_NOTES_WHERE_TITLE_QS, "%"+filterInput+"%")
}

func textFilterHandler(writer http.ResponseWriter, request *http.Request, filterInput string) {
	filteredIndexHandler(writer, request, manager.SELECT_NOTES_WHERE_TEXT_QS, "%"+filterInput+"%")
}

func bothFilterHandler(writer http.ResponseWriter, request *http.Request, filterInput string) {
	filteredIndexHandler(writer, request, manager.SELECT_NOTES_WHERE_BOTH_QS, "%"+filterInput+"%", "%"+filterInput+"%")
}

var validNotePath = regexp.MustCompile("^/(Note|ConfirmDeleteNote|SaveNote|ConfirmPasteBinNote)/([0-9]+)$")

func makeNoteIDHandler(function func(http.ResponseWriter, *http.Request, int)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
                if tooManyRequests() {
                        http.Error(writer, "Slow down, buddy!", 429)
                        return
                }
                
		urlTokens := validNotePath.FindStringSubmatch(request.URL.Path)
		if urlTokens == nil {
			http.NotFound(writer, request)
			return
		}
		id, _ := strconv.Atoi(urlTokens[2])
		function(writer, request, id)
	}
}

var validFilterPath = regexp.MustCompile("^/(Title|Text|Both)Filter/([0-9a-zA-Z ]+)$")

func makeFilterHandler(function func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
                if tooManyRequests() {
                        http.Error(writer, "Slow down, buddy!", 429)
                        return
                }
                
		urlTokens := validFilterPath.FindStringSubmatch(request.URL.Path)
		if urlTokens == nil {
			http.NotFound(writer, request)
			return
		}
		function(writer, request, urlTokens[2])
	}
}

var validPreparePostPath = regexp.MustCompile("^/(DeleteNote|EditNote|PasteBinNote)/([0-9]+)$")

func makePreparePostHandler(function func(http.ResponseWriter, *http.Request, string, int)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
                if tooManyRequests() {
                        http.Error(writer, "Slow down, buddy!", 429)
                        return
                }
                
		urlTokens := validPreparePostPath.FindStringSubmatch(request.URL.Path)
		if urlTokens == nil {
			http.NotFound(writer, request)
			return
		}
		id, _ := strconv.Atoi(urlTokens[2])
		function(writer, request, urlTokens[1], id)
	}
}

var validPath = regexp.MustCompile("(^/(AddNote|NewNote)/$|/)")

func makeHandler(function func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
                if tooManyRequests() {
                        http.Error(writer, "Slow down, buddy!", 429)
                        return
                }
                
		urlTokens := validPath.FindStringSubmatch(request.URL.Path)
		if urlTokens == nil {
			http.NotFound(writer, request)
			return
		}
                
		function(writer, request)
	}
}

var lastRequestTime int64 = math.MinInt64

func tooManyRequests() bool {
        var currentTime int64 = time.Now().UnixNano()
        
        if lastRequestTime > math.MinInt64 && currentTime - lastRequestTime <= TOO_MANY_REQUEST_TIME_SPAN_IN_NS {
                return true
        }
        
        lastRequestTime = currentTime
        return false
}

func main() {
	http.HandleFunc("/", makeHandler(indexHandler))
	http.HandleFunc("/AddNote/", makeHandler(addNoteHandler))
	http.HandleFunc("/NewNote/", makeHandler(newNoteHandler))
        
        http.HandleFunc("/EditNote/", makePreparePostHandler(preparePostHandler))
	http.HandleFunc("/DeleteNote/", makePreparePostHandler(preparePostHandler))
	http.HandleFunc("/PasteBinNote/", makePreparePostHandler(preparePostHandler))
        
	http.HandleFunc("/Note/", makeNoteIDHandler(noteDetailsHandler))
	http.HandleFunc("/SaveNote/", makeNoteIDHandler(saveNoteHandler))
	http.HandleFunc("/ConfirmDeleteNote/", makeNoteIDHandler(deleteNoteHandler))
	http.HandleFunc("/ConfirmPasteBinNote/", makeNoteIDHandler(pasteBinNoteHandler))
        
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

	err = http.ListenAndServe(":8080", nil)
        
        if err != nil {
                log.Fatal(err)
                return
        }
}
