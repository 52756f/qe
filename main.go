package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/gdamore/tcell/v2"
)

type promptKind int

const (
	promptNone     promptKind = iota
	promptFilename            // Strg+S ohne Dateiname
	promptSaveExit            // Strg+X mit ungespeichertem Inhalt
	promptSearch              // Strg+F Suche
)

// Editor repräsentiert den Zustand unseres Texteditors
type Editor struct {
	screen        tcell.Screen
	lines         [][]rune // Der Text, gespeichert als Slice von Zeilen (Runen)
	cursorX       int      // Aktuelle Cursor-Spalte
	cursorY       int      // Aktuelle Cursor-Zeile
	scrollY       int      // Erste sichtbare Zeile (vertikales Scrollen)
	filename      string   // Geöffnete Datei (leer wenn keine)
	dirty         bool     // Ungespeicherte Änderungen vorhanden
	clipboard     [][]rune // Kopierte Zeilen (F5/F6)
	selActive     bool     // Auswahl aktiv
	selAnchorX    int      // Auswahl-Anker Spalte
	selAnchorY    int      // Auswahl-Anker Zeile
	searchTerm    string   // Letzter Suchbegriff
	prompt        promptKind
	promptInput   []rune
	exitAfterSave bool
	hlStyles      [][]tcell.Style // Syntax-Highlighting-Stile pro Zeichen
	hlDirty       bool            // Highlighting muss neu berechnet werden
}

// tokenStyle bildet chroma-Tokentypen auf tcell-Stile ab
func tokenStyle(tt chroma.TokenType) tcell.Style {
	s := tcell.StyleDefault
	switch {
	case tt.InCategory(chroma.Keyword):
		return s.Foreground(tcell.ColorBlue).Bold(true)
	case tt.InCategory(chroma.LiteralString):
		return s.Foreground(tcell.ColorGreen)
	case tt.InCategory(chroma.Comment):
		return s.Foreground(tcell.ColorGray)
	case tt.InCategory(chroma.LiteralNumber):
		return s.Foreground(tcell.ColorYellow)
	case tt.InCategory(chroma.NameFunction):
		return s.Foreground(tcell.ColorAqua)
	case tt.InCategory(chroma.NameBuiltin):
		return s.Foreground(tcell.ColorFuchsia)
	default:
		return s
	}
}

// buildHighlights tokenisiert den gesamten Puffer und baut das Stil-Raster auf
func (e *Editor) buildHighlights() {
	e.hlDirty = false

	e.hlStyles = make([][]tcell.Style, len(e.lines))
	for i, line := range e.lines {
		e.hlStyles[i] = make([]tcell.Style, len(line))
	}

	if e.filename == "" {
		return
	}
	lexer := lexers.Match(e.filename)
	if lexer == nil {
		return
	}
	lexer = chroma.Coalesce(lexer)

	var sb strings.Builder
	for i, line := range e.lines {
		sb.WriteString(string(line))
		if i < len(e.lines)-1 {
			sb.WriteByte('\n')
		}
	}

	iterator, err := lexer.Tokenise(nil, sb.String())
	if err != nil {
		return
	}

	row, col := 0, 0
	for tok := iterator(); tok != chroma.EOF; tok = iterator() {
		style := tokenStyle(tok.Type)
		for _, ch := range tok.Value {
			if ch == '\n' {
				row++
				col = 0
			} else {
				if row < len(e.hlStyles) && col < len(e.hlStyles[row]) {
					e.hlStyles[row][col] = style
				}
				col++
			}
		}
	}
}

// saveFile schreibt den Puffer in die geöffnete Datei
func (e *Editor) saveFile() error {
	if e.filename == "" {
		return nil
	}
	var sb strings.Builder
	for i, line := range e.lines {
		sb.WriteString(string(line))
		if i < len(e.lines)-1 {
			sb.WriteByte('\n')
		}
	}
	err := os.WriteFile(e.filename, []byte(sb.String()), 0644)
	if err == nil {
		e.dirty = false
	}
	return err
}

// loadFile liest eine Datei und gibt den Inhalt als Zeilen zurück
func loadFile(filename string) ([][]rune, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	raw := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(raw) > 0 && raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}
	lines := make([][]rune, len(raw))
	for i, l := range raw {
		lines[i] = []rune(l)
	}
	if len(lines) == 0 {
		lines = [][]rune{{}}
	}
	return lines, nil
}

// searchFrom sucht term ab (startY, startX) vorwärts mit Wraparound.
func (e *Editor) searchFrom(term string, startY, startX int) bool {
	n := len(e.lines)
	for i := 0; i < n; i++ {
		y := (startY + i) % n
		line := string(e.lines[y])
		sx := 0
		if i == 0 {
			if startX > len(line) {
				startX = len(line)
			}
			sx = startX
		}
		if idx := strings.Index(line[sx:], term); idx >= 0 {
			e.cursorY = y
			e.cursorX = sx + idx
			return true
		}
	}
	return false
}

// selBounds gibt Start und Ende der Auswahl in Pufferkoordinaten zurück
func (e *Editor) selBounds() (startY, startX, endY, endX int) {
	ay, ax := e.selAnchorY, e.selAnchorX
	cy, cx := e.cursorY, e.cursorX
	if ay < cy || (ay == cy && ax <= cx) {
		return ay, ax, cy, cx
	}
	return cy, cx, ay, ax
}

// isInSelection prüft ob Position (y,x) innerhalb der Auswahl liegt
func (e *Editor) isInSelection(y, x int) bool {
	sy, sx, ey, ex := e.selBounds()
	if y < sy || y > ey {
		return false
	}
	if y == sy && x < sx {
		return false
	}
	if y == ey && x >= ex {
		return false
	}
	return true
}

// deleteSelection löscht den ausgewählten Text und setzt den Cursor an den Auswahlstart
func (e *Editor) deleteSelection() {
	sy, sx, ey, ex := e.selBounds()
	if sy == ey && sx == ex {
		e.selActive = false
		return
	}
	suffix := append([]rune{}, e.lines[ey][ex:]...)
	e.lines[sy] = append(e.lines[sy][:sx], suffix...)
	e.lines = append(e.lines[:sy+1], e.lines[ey+1:]...)
	if len(e.lines) == 0 {
		e.lines = [][]rune{{}}
	}
	e.cursorY = sy
	e.cursorX = sx
	e.selActive = false
	e.dirty = true
	e.hlDirty = true
}

// selectedText gibt den ausgewählten Text als Slice von Zeilen zurück
func (e *Editor) selectedText() [][]rune {
	sy, sx, ey, ex := e.selBounds()
	if sy == ey {
		return [][]rune{append([]rune{}, e.lines[sy][sx:ex]...)}
	}
	result := make([][]rune, 0, ey-sy+1)
	result = append(result, append([]rune{}, e.lines[sy][sx:]...))
	for y := sy + 1; y < ey; y++ {
		result = append(result, append([]rune{}, e.lines[y]...))
	}
	result = append(result, append([]rune{}, e.lines[ey][:ex]...))
	return result
}

func main() {
	editor := &Editor{
		lines:   [][]rune{{}},
		hlDirty: true,
	}

	if len(os.Args) > 1 {
		editor.filename = os.Args[1]
		lines, err := loadFile(editor.filename)
		if err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Fehler beim Öffnen der Datei: %v\n", err)
			os.Exit(1)
		}
		if err == nil {
			editor.lines = lines
		}
	}

	screen, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fehler beim Erstellen des Screens: %v\n", err)
		os.Exit(1)
	}
	if err := screen.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Fehler beim Initialisieren des Terminals: %v\n", err)
		os.Exit(1)
	}
	screen.EnableMouse()

	editor.screen = screen
	defer screen.Fini()

	for {
		editor.Draw()
		screen.Show()

		ev := screen.PollEvent()

		switch ev := ev.(type) {
		case *tcell.EventKey:
			if editor.prompt != promptNone {
				if editor.HandlePrompt(ev) {
					return
				}
				continue
			}
			switch ev.Key() {
			case tcell.KeyCtrlX:
				if editor.dirty {
					editor.prompt = promptSaveExit
				} else {
					return
				}
			case tcell.KeyCtrlS:
				if editor.filename == "" {
					editor.prompt = promptFilename
				} else {
					editor.saveFile()
				}
			case tcell.KeyCtrlF:
				editor.prompt = promptSearch
				editor.promptInput = []rune(editor.searchTerm)
			case tcell.KeyCtrlA:
				editor.selActive = true
				editor.selAnchorX = 0
				editor.selAnchorY = 0
				editor.cursorY = len(editor.lines) - 1
				editor.cursorX = len(editor.lines[editor.cursorY])
			default:
				editor.HandleEvent(ev)
			}
		case *tcell.EventMouse:
			if ev.Buttons() == tcell.Button1 {
				col, row := ev.Position()
				_, height := screen.Size()
				if row >= 1 && row < height-1 {
					editor.selActive = false
					y := row - 1 + editor.scrollY
					if y >= len(editor.lines) {
						y = len(editor.lines) - 1
					}
					editor.cursorY = y
					x := col
					if x > len(editor.lines[y]) {
						x = len(editor.lines[y])
					}
					editor.cursorX = x
				}
			}
		case *tcell.EventResize:
			screen.Sync()
		}
	}
}

// HandlePrompt verarbeitet Eingaben während ein Prompt aktiv ist.
// Gibt true zurück wenn der Editor beendet werden soll.
func (e *Editor) HandlePrompt(ev *tcell.EventKey) bool {
	switch e.prompt {
	case promptSaveExit:
		switch ev.Key() {
		case tcell.KeyRune:
			switch ev.Rune() {
			case 'j', 'J', 'y', 'Y':
				if e.filename == "" {
					e.prompt = promptFilename
					e.exitAfterSave = true
				} else {
					e.saveFile()
					return true
				}
			case 'n', 'N':
				return true
			}
		case tcell.KeyEscape:
			e.prompt = promptNone
		}

	case promptFilename:
		switch ev.Key() {
		case tcell.KeyEnter:
			if len(e.promptInput) > 0 {
				e.filename = string(e.promptInput)
				e.promptInput = nil
				e.prompt = promptNone
				e.hlDirty = true
				e.saveFile()
				if e.exitAfterSave {
					e.exitAfterSave = false
					return true
				}
			}
		case tcell.KeyEscape:
			e.promptInput = nil
			e.prompt = promptNone
			e.exitAfterSave = false
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if len(e.promptInput) > 0 {
				e.promptInput = e.promptInput[:len(e.promptInput)-1]
			}
		case tcell.KeyRune:
			e.promptInput = append(e.promptInput, ev.Rune())
		}

	case promptSearch:
		switch ev.Key() {
		case tcell.KeyEnter:
			term := string(e.promptInput)
			if term == "" {
				term = e.searchTerm
			}
			if term != "" {
				startX := e.cursorX
				if term == e.searchTerm {
					startX = e.cursorX + 1
				}
				e.searchTerm = term
				e.searchFrom(term, e.cursorY, startX)
			}
			e.promptInput = nil
			e.prompt = promptNone
			e.selActive = false
		case tcell.KeyEscape:
			e.promptInput = nil
			e.prompt = promptNone
			e.selActive = false
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if len(e.promptInput) > 0 {
				e.promptInput = e.promptInput[:len(e.promptInput)-1]
			}
		case tcell.KeyRune:
			e.promptInput = append(e.promptInput, ev.Rune())
		}
	}
	return false
}

// HandleEvent verarbeitet Tasteneingaben und ändert den Textpuffer
func (e *Editor) HandleEvent(ev *tcell.EventKey) {
	if e.selActive {
		switch ev.Key() {
		case tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyDelete, tcell.KeyRune, tcell.KeyEnter, tcell.KeyF5:
			// diese Tasten verwalten die Auswahl selbst
		default:
			e.selActive = false
		}
	}

	switch ev.Key() {
	case tcell.KeyEnter:
		if e.selActive {
			e.deleteSelection()
		}
		currentLine := e.lines[e.cursorY]
		leftPart := append([]rune{}, currentLine[:e.cursorX]...)
		rightPart := append([]rune{}, currentLine[e.cursorX:]...)
		e.lines[e.cursorY] = leftPart
		e.lines = append(e.lines, nil)
		copy(e.lines[e.cursorY+2:], e.lines[e.cursorY+1:])
		e.lines[e.cursorY+1] = rightPart
		e.cursorY++
		e.cursorX = 0
		e.dirty = true
		e.hlDirty = true

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if e.selActive {
			e.deleteSelection()
			return
		}
		if e.cursorX > 0 {
			line := e.lines[e.cursorY]
			e.lines[e.cursorY] = append(line[:e.cursorX-1], line[e.cursorX:]...)
			e.cursorX--
			e.dirty = true
			e.hlDirty = true
		} else if e.cursorY > 0 {
			prevLine := e.lines[e.cursorY-1]
			currentLine := e.lines[e.cursorY]
			e.cursorX = len(prevLine)
			e.cursorY--
			e.lines[e.cursorY] = append(prevLine, currentLine...)
			e.lines = append(e.lines[:e.cursorY+1], e.lines[e.cursorY+2:]...)
			e.dirty = true
			e.hlDirty = true
		}

	case tcell.KeyDelete:
		if e.selActive {
			e.deleteSelection()
			return
		}
		line := e.lines[e.cursorY]
		if e.cursorX < len(line) {
			e.lines[e.cursorY] = append(line[:e.cursorX], line[e.cursorX+1:]...)
			e.dirty = true
			e.hlDirty = true
		} else if e.cursorY < len(e.lines)-1 {
			next := e.lines[e.cursorY+1]
			e.lines[e.cursorY] = append(line, next...)
			e.lines = append(e.lines[:e.cursorY+1], e.lines[e.cursorY+2:]...)
			e.dirty = true
			e.hlDirty = true
		}

	case tcell.KeyHome:
		e.cursorX = 0
	case tcell.KeyEnd:
		e.cursorX = len(e.lines[e.cursorY])

	case tcell.KeyPgUp:
		_, height := e.screen.Size()
		textHeight := height - 2
		e.cursorY -= textHeight
		if e.cursorY < 0 {
			e.cursorY = 0
		}
		if e.cursorX > len(e.lines[e.cursorY]) {
			e.cursorX = len(e.lines[e.cursorY])
		}

	case tcell.KeyPgDn:
		_, height := e.screen.Size()
		textHeight := height - 2
		e.cursorY += textHeight
		if e.cursorY >= len(e.lines) {
			e.cursorY = len(e.lines) - 1
		}
		if e.cursorX > len(e.lines[e.cursorY]) {
			e.cursorX = len(e.lines[e.cursorY])
		}

	case tcell.KeyLeft:
		if e.cursorX > 0 {
			e.cursorX--
		}
	case tcell.KeyRight:
		if e.cursorX < len(e.lines[e.cursorY]) {
			e.cursorX++
		}
	case tcell.KeyUp:
		if e.cursorY > 0 {
			e.cursorY--
			if e.cursorX > len(e.lines[e.cursorY]) {
				e.cursorX = len(e.lines[e.cursorY])
			}
		}
	case tcell.KeyDown:
		if e.cursorY < len(e.lines)-1 {
			e.cursorY++
			if e.cursorX > len(e.lines[e.cursorY]) {
				e.cursorX = len(e.lines[e.cursorY])
			}
		}

	case tcell.KeyF5:
		if e.selActive {
			e.clipboard = e.selectedText()
			e.selActive = false
		} else {
			e.clipboard = [][]rune{append([]rune{}, e.lines[e.cursorY]...)}
		}

	case tcell.KeyF6:
		if e.clipboard == nil {
			break
		}
		if len(e.clipboard) == 1 {
			newLine := append([]rune{}, e.clipboard[0]...)
			e.lines = append(e.lines, nil)
			copy(e.lines[e.cursorY+1:], e.lines[e.cursorY:])
			e.lines[e.cursorY] = newLine
			e.cursorX = 0
		} else {
			n := len(e.clipboard)
			curLine := e.lines[e.cursorY]
			before := append([]rune{}, curLine[:e.cursorX]...)
			after := append([]rune{}, curLine[e.cursorX:]...)
			newLines := make([][]rune, n)
			newLines[0] = append(before, e.clipboard[0]...)
			for i := 1; i < n-1; i++ {
				newLines[i] = append([]rune{}, e.clipboard[i]...)
			}
			newLines[n-1] = append(append([]rune{}, e.clipboard[n-1]...), after...)
			rest := append([][]rune{}, e.lines[e.cursorY+1:]...)
			e.lines = append(e.lines[:e.cursorY], append(newLines, rest...)...)
			e.cursorY += n - 1
			e.cursorX = len(e.clipboard[n-1])
		}
		e.dirty = true
		e.hlDirty = true

	case tcell.KeyF8:
		if len(e.lines) == 1 {
			e.lines[0] = []rune{}
		} else {
			e.lines = append(e.lines[:e.cursorY], e.lines[e.cursorY+1:]...)
			if e.cursorY >= len(e.lines) {
				e.cursorY = len(e.lines) - 1
			}
		}
		if e.cursorX > len(e.lines[e.cursorY]) {
			e.cursorX = len(e.lines[e.cursorY])
		}
		e.dirty = true
		e.hlDirty = true

	case tcell.KeyRune:
		if e.selActive {
			e.deleteSelection()
		}
		ch := ev.Rune()
		line := e.lines[e.cursorY]
		e.lines[e.cursorY] = append(line[:e.cursorX], append([]rune{ch}, line[e.cursorX:]...)...)
		e.cursorX++
		e.dirty = true
		e.hlDirty = true
	}
}

// Draw zeichnet den Text und den Cursor auf den Bildschirm
func (e *Editor) Draw() {
	if e.hlDirty {
		e.buildHighlights()
	}

	e.screen.Clear()
	width, height := e.screen.Size()
	textHeight := height - 2

	// Scrollposition anpassen
	if e.cursorY < e.scrollY {
		e.scrollY = e.cursorY
	}
	if e.cursorY >= e.scrollY+textHeight {
		e.scrollY = e.cursorY - textHeight + 1
	}

	barStyle := tcell.StyleDefault.Reverse(true)

	// Obere Statusleiste
	name := e.filename
	if name == "" {
		name = "Neue Datei"
	}
	if e.dirty {
		name += " *"
	}
	topMsg := fmt.Sprintf(" %s  Line %d/%d ", name, e.cursorY+1, len(e.lines))
	drawBar(e.screen, 0, width, topMsg, barStyle)

	// Untere Leiste
	var bottomMsg string
	switch e.prompt {
	case promptSaveExit:
		bottomMsg = " Änderungen speichern? [J=Ja  N=Nein  Esc=Abbrechen] "
	case promptFilename:
		bottomMsg = " Dateiname: " + string(e.promptInput)
	case promptSearch:
		bottomMsg = " Suchen: " + string(e.promptInput)
	default:
		bottomMsg = " ^S Speichern   ^X Beenden   ^F Suchen   ^A Alles auswählen   F5 Kopieren   F6 Einfügen   F8 Zeile löschen   Del Vorwärts löschen   Home Zeilenanfang   End Zeilenende   PgUp/PgDn Seite "
	}
	drawBar(e.screen, height-1, width, bottomMsg, barStyle)

	// Textinhalt zeichnen
	for i := 0; i < textHeight; i++ {
		lineIdx := e.scrollY + i
		if lineIdx >= len(e.lines) {
			break
		}
		screenY := i + 1
		for x, ch := range e.lines[lineIdx] {
			if x >= width {
				break
			}
			style := tcell.StyleDefault
			if e.hlStyles != nil && lineIdx < len(e.hlStyles) && x < len(e.hlStyles[lineIdx]) {
				style = e.hlStyles[lineIdx][x]
			}
			if e.selActive && e.isInSelection(lineIdx, x) {
				style = style.Reverse(true)
			}
			e.screen.SetContent(x, screenY, ch, nil, style)
		}
	}

	// Cursor positionieren
	switch e.prompt {
	case promptFilename:
		cx := len([]rune(" Dateiname: ")) + len(e.promptInput)
		if cx < width {
			e.screen.ShowCursor(cx, height-1)
		} else {
			e.screen.HideCursor()
		}
	case promptSearch:
		cx := len([]rune(" Suchen: ")) + len(e.promptInput)
		if cx < width {
			e.screen.ShowCursor(cx, height-1)
		} else {
			e.screen.HideCursor()
		}
	default:
		screenCursorY := e.cursorY - e.scrollY + 1
		if e.cursorX < width && screenCursorY >= 1 && screenCursorY <= textHeight {
			e.screen.ShowCursor(e.cursorX, screenCursorY)
		} else {
			e.screen.HideCursor()
		}
	}
}

// drawBar füllt eine horizontale Zeile mit msg (aufgefüllt bis width)
func drawBar(screen tcell.Screen, y, width int, msg string, style tcell.Style) {
	runes := []rune(msg)
	for x := 0; x < width; x++ {
		ch := ' '
		if x < len(runes) {
			ch = runes[x]
		}
		screen.SetContent(x, y, ch, nil, style)
	}
}
