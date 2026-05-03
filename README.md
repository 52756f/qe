# qe

A minimal terminal text editor written in Go.

## Features

- Character insertion and deletion (Backspace with line-merge)
- Enter to split lines
- Arrow key navigation
- Cross-platform terminal I/O via [tcell](https://github.com/gdamore/tcell)

## Usage

```bash
go run main.go        # run directly
go build -o qe .      # build binary
./qe                  # launch editor
```

Quit with **Ctrl+X**.

## Requirements

- Go 1.18+

## Dependencies

- [`github.com/gdamore/tcell/v2`](https://github.com/gdamore/tcell)
