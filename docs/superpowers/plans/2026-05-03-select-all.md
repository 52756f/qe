# Select-All Visual Selection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Ctrl+A select-all with visual highlight, selection-aware copy/delete/type-to-replace, and basic mouse click support to the qe terminal text editor.

**Architecture:** All state lives in the single `Editor` struct in `main.go`. Three new fields (`selActive`, `selAnchorX`, `selAnchorY`) track the anchor end of the selection; the cursor is the live end. Four helper methods (`selBounds`, `isInSelection`, `deleteSelection`, `selectedText`) encapsulate selection logic. The clipboard type changes from `[]rune` to `[][]rune`. All changes are in `main.go`.

**Tech Stack:** Go, github.com/gdamore/tcell/v2

---

### Task 1: Add selection fields, helpers, and update clipboard type

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Add fields to Editor struct**

In `main.go`, change the `Editor` struct. Replace:

```go
clipboard     []rune   // Kopierte Zeile (F5/F6)
```

With:

```go
clipboard     [][]rune // Kopierte Zeilen (F5/F6)
selActive     bool     // Auswahl aktiv
selAnchorX    int      // Auswahl-Anker Spalte
selAnchorY    int      // Auswahl-Anker Zeile
```

- [ ] **Step 2: Add helper methods after the `searchFrom` function**

Add these four methods after the closing brace of `searchFrom` and before `func main()`:

```go
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
```

- [ ] **Step 3: Update F5 to use [][]rune clipboard**

In `HandleEvent`, replace the `tcell.KeyF5` case:

```go
case tcell.KeyF5:
	e.clipboard = append([]rune{}, e.lines[e.cursorY]...)
```

With:

```go
case tcell.KeyF5:
	if e.selActive {
		e.clipboard = e.selectedText()
		e.selActive = false
	} else {
		e.clipboard = [][]rune{append([]rune{}, e.lines[e.cursorY]...)}
	}
```

- [ ] **Step 4: Update F6 to handle [][]rune clipboard**

In `HandleEvent`, replace the entire `tcell.KeyF6` case:

```go
case tcell.KeyF6:
	if e.clipboard == nil {
		break
	}
	newLine := append([]rune{}, e.clipboard...)
	e.lines = append(e.lines, nil)
	copy(e.lines[e.cursorY+1:], e.lines[e.cursorY:])
	e.lines[e.cursorY] = newLine
	e.cursorX = 0
	e.dirty = true
	e.hlDirty = true
```

With:

```go
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
```

- [ ] **Step 5: Build and verify no errors**

```bash
go build -o qe .
```

Expected: no output, binary `qe` produced.

- [ ] **Step 6: Commit**

```bash
git add main.go
git commit -m "feat: add selection fields, helpers and update clipboard to [][]rune"
```

---

### Task 2: Implement Ctrl+A and selection-aware key handling

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Add Ctrl+A in the main event loop**

In `main()`, in the `switch ev.Key()` block, add a new case before `default`:

```go
case tcell.KeyCtrlA:
	editor.selActive = true
	editor.selAnchorX = 0
	editor.selAnchorY = 0
	editor.cursorY = len(editor.lines) - 1
	editor.cursorX = len(editor.lines[editor.cursorY])
```

- [ ] **Step 2: Clear selection on non-selection-aware keys**

At the very top of `HandleEvent`, before the `switch ev.Key()` statement, add:

```go
if e.selActive {
	switch ev.Key() {
	case tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyDelete, tcell.KeyRune:
		// diese Tasten verwalten die Auswahl selbst
	default:
		e.selActive = false
	}
}
```

- [ ] **Step 3: Make Backspace selection-aware**

In the `tcell.KeyBackspace, tcell.KeyBackspace2` case in `HandleEvent`, add at the very top of that case:

```go
if e.selActive {
	e.deleteSelection()
	return
}
```

So the case now reads:

```go
case tcell.KeyBackspace, tcell.KeyBackspace2:
	if e.selActive {
		e.deleteSelection()
		return
	}
	if e.cursorX > 0 {
	// ... rest unchanged
```

- [ ] **Step 4: Make Delete selection-aware**

In the `tcell.KeyDelete` case in `HandleEvent`, add at the very top:

```go
if e.selActive {
	e.deleteSelection()
	return
}
```

So the case now reads:

```go
case tcell.KeyDelete:
	if e.selActive {
		e.deleteSelection()
		return
	}
	line := e.lines[e.cursorY]
	// ... rest unchanged
```

- [ ] **Step 5: Make rune insertion selection-aware**

In the `tcell.KeyRune` case in `HandleEvent`, add at the very top:

```go
if e.selActive {
	e.deleteSelection()
}
```

So the case now reads:

```go
case tcell.KeyRune:
	if e.selActive {
		e.deleteSelection()
	}
	ch := ev.Rune()
	line := e.lines[e.cursorY]
	// ... rest unchanged
```

- [ ] **Step 6: Build and verify no errors**

```bash
go build -o qe .
```

Expected: no output.

- [ ] **Step 7: Manual test — Ctrl+A basics**

```bash
./qe main.go
```

- Press Ctrl+A. The cursor should jump to the last line.
- Press any arrow key. The selection should clear (no visible change yet — highlight comes in Task 3).
- Press Ctrl+A then Backspace. All text should be deleted, leaving an empty buffer.
- Press Ctrl+X N to exit without saving.

- [ ] **Step 8: Commit**

```bash
git add main.go
git commit -m "feat: implement Ctrl+A and selection-aware key handling"
```

---

### Task 3: Draw selection highlight

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Apply reverse style for selected characters in Draw()**

In `Draw()`, find the inner character-drawing loop:

```go
for x, ch := range e.lines[lineIdx] {
    if x >= width {
        break
    }
    style := tcell.StyleDefault
    if e.hlStyles != nil && lineIdx < len(e.hlStyles) && x < len(e.hlStyles[lineIdx]) {
        style = e.hlStyles[lineIdx][x]
    }
    e.screen.SetContent(x, screenY, ch, nil, style)
}
```

Replace with:

```go
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
```

- [ ] **Step 2: Build and verify no errors**

```bash
go build -o qe .
```

Expected: no output.

- [ ] **Step 3: Manual test — visual highlight**

```bash
./qe main.go
```

- Press Ctrl+A. All text should be highlighted (reversed colours) from the first character to the end of the last line.
- Press an arrow key. Highlight should disappear.
- Press Ctrl+A then F5 to copy, then navigate to a new position, then F6 to paste. The entire file contents should be inserted.
- Press Ctrl+X N to exit without saving.

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat: draw visual selection highlight"
```

---

### Task 4: Mouse support and bottom bar update

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Enable mouse during initialisation**

In `main()`, after `screen.Init()` succeeds (after the `if err := screen.Init()` block), add:

```go
screen.EnableMouse()
```

- [ ] **Step 2: Handle mouse click events in the event loop**

In `main()`, in the outer `switch ev := ev.(type)` block, add a new case after the `*tcell.EventKey` case:

```go
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
```

- [ ] **Step 3: Add Ctrl+A hint to bottom bar**

In `Draw()`, replace the default `bottomMsg` line:

```go
bottomMsg = " ^S Speichern   ^X Beenden   ^F Suchen   F5 Kopieren   F6 Einfügen   F8 Zeile löschen   Del Vorwärts löschen   Home Zeilenanfang   End Zeilenende   PgUp/PgDn Seite "
```

With:

```go
bottomMsg = " ^S Speichern   ^X Beenden   ^F Suchen   ^A Alles auswählen   F5 Kopieren   F6 Einfügen   F8 Zeile löschen   Del Vorwärts löschen   Home Zeilenanfang   End Zeilenende   PgUp/PgDn Seite "
```

- [ ] **Step 4: Build and verify no errors**

```bash
go build -o qe .
```

Expected: no output.

- [ ] **Step 5: Manual test — mouse and bottom bar**

```bash
./qe main.go
```

- Confirm `^A Alles auswählen` appears in the bottom status bar.
- Press Ctrl+A to select all (text highlighted).
- Click anywhere in the text area with the mouse. The highlight should clear and the cursor should move to the clicked position.
- Click in the top or bottom status bar — nothing should happen (no crash, no cursor move).

- [ ] **Step 6: Commit**

```bash
git add main.go
git commit -m "feat: add mouse click support and bottom bar hint for Ctrl+A"
```

---

### Task 5: Push to GitHub

- [ ] **Step 1: Push all commits**

```bash
git push
```

Expected: all four feature commits appear on `https://github.com/52756f/qe`.
