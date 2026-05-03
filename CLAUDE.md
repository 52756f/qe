# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go run main.go      # run the editor
go build            # compile to ./qe binary
go build -o qe .    # explicit output name
```

There are no tests yet. Quit the running editor with `Ctrl+X`.

## Architecture

`qe` is a minimal terminal text editor — a single `main.go` file, no packages.

**`Editor` struct** — all mutable state lives here:
- `screen tcell.Screen` — the tcell terminal handle
- `lines [][]rune` — the text buffer, one `[]rune` per line
- `cursorX`, `cursorY` — cursor position in buffer coordinates

**Event loop** (`main`): initializes a `tcell.Screen`, then loops: `Draw` → `screen.Show` → `PollEvent` → dispatch. `EventKey` is routed to `HandleEvent`; `EventResize` calls `screen.Sync`.

**`HandleEvent`** — mutates `lines` and the cursor in response to keypresses. Supports: character insertion, Backspace (with line-merge), Enter (line-split), and arrow keys. Line-splits and merges operate directly on the `[]rune` slices.

**`Draw`** — clears the screen, iterates `lines` writing each rune via `screen.SetContent`, then positions the cursor with `screen.ShowCursor`. No scrolling yet; content beyond the terminal dimensions is clipped.

**Dependency**: `github.com/gdamore/tcell/v2` for cross-platform terminal I/O.

## Notes

- Code comments are written in German.
- Scrolling is not yet implemented — the cursor is hidden if it moves off-screen.
