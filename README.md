# qe

A minimal terminal text editor written in Go.

```bash
go install github.com/52756f/qe@latest
```

Or clone and build manually (e.g. on a VPS):

```bash
git clone https://github.com/52756f/qe
cd qe
go build -buildvcs=false -o qe .
sudo mv qe /usr/local/bin/
```

## Features

- Character insertion and deletion (Backspace with line-merge)
- Enter to split lines
- Arrow key navigation
- **Ctrl+A** — select all with visual highlight
- Selection-aware editing: copy (Ctrl+C), delete, type-to-replace
- Mouse click to reposition cursor and clear selection
- Syntax highlighting via [chroma](https://github.com/alecthomas/chroma)
- Search with **Ctrl+F**
- Save with **Ctrl+S**, save as with **Ctrl+W**, quit with **Ctrl+X**
- Line copy/paste (Ctrl+C / Ctrl+V), line delete (F8)
- Undo with **Ctrl+Z**
- Page Up/Down, Home/End
- Cross-platform terminal I/O via [tcell](https://github.com/gdamore/tcell)

## Key Bindings

| Key | Action |
|-----|--------|
| Ctrl+A | Select all |
| Ctrl+S | Save |
| Ctrl+W | Save as |
| Ctrl+X | Quit |
| Ctrl+Z | Undo |
| Ctrl+F | Search |
| Ctrl+C | Copy line (or selection) |
| Ctrl+V | Paste |
| F8 | Delete line |
| Backspace / Delete | Delete character (or selection) |
| Arrow keys | Move cursor |
| Home / End | Start / end of line |
| Page Up / Down | Scroll by page |

## Usage

```bash
go run main.go        # run directly
go build -o qe .      # build binary
./qe [file]           # launch editor, optionally opening a file
```

## Requirements

- Go 1.21+

## Dependencies

- [`github.com/gdamore/tcell/v2`](https://github.com/gdamore/tcell)
- [`github.com/alecthomas/chroma/v2`](https://github.com/alecthomas/chroma)
