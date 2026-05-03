# Select-All & Visual Selection Design

**Date:** 2026-05-03
**Status:** Approved

## Goal

Add Ctrl+A (select all) with visual highlight to `qe`. Selection enables copy, delete, and type-to-replace. Also adds basic mouse click support to clear selection and reposition the cursor.

## Data Model

Three new fields on `Editor`:

```go
selActive  bool
selAnchorX int
selAnchorY int
```

The clipboard field changes type from `[]rune` to `[][]rune` to support multi-line copy/paste.

Selection spans from `(selAnchorY, selAnchorX)` to `(cursorY, cursorX)`. The canonical start is whichever endpoint comes first in the buffer; the canonical end is the other.

## Key Bindings

| Key | Behavior |
|-----|----------|
| Ctrl+A | Set `selActive=true`, anchor=(0,0), cursor=end of last line |
| F5 (with selection) | Copy selected text into `[][]rune` clipboard |
| F5 (no selection) | Existing behavior: copy current line |
| F6 | Paste clipboard; multi-line pastes insert all lines at cursor |
| Backspace / Delete (with selection) | Delete selected text; cursor lands at selection start |
| Printable rune (with selection) | Delete selected text, insert typed character |
| Any other key (with selection) | Clear selection, then handle key normally |

Bottom bar hint added: `^A Alles auswählen`

## Mouse Support

- `screen.EnableMouse()` called during initialization.
- `*tcell.EventMouse` handled in the event loop.
- Left-button press: clear `selActive`, move cursor to clicked position.
  - `cursorY = clamp(screenRow - 1 + scrollY, 0, len(lines)-1)`
  - `cursorX = clamp(screenCol, 0, len(lines[cursorY]))`
- Other mouse buttons: ignored.

## Draw — Selection Highlight

In `Draw()`, for each character in the text area, if `selActive` and the character falls within the selection range, apply `.Reverse(true)` to its style (layered on top of syntax highlighting).

Selection range check: a position `(y, x)` is selected when it falls between the canonical start and end (inclusive of start, exclusive of end).

## Clipboard — Multi-line Paste (F6)

F5 without a selection stores `[][]rune{{currentLine}}` (single-element slice), keeping paste behaviour identical to today.

When the clipboard contains a single line, F6 inserts it at the cursor row — the existing line at that row shifts down. When it contains multiple lines, F6 splits the current line at `cursorX`, inserts all clipboard lines, and places the cursor at the end of the last pasted line.

## Out of Scope

- Shift+Arrow partial selection (can be added later; anchor model supports it)
- Mouse drag selection
- System clipboard integration
