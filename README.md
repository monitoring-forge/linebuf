# linebuf

A fast, bounded-memory line scanner for Go.

## Features

- Callback and Go 1.23 iterator APIs
- Configurable initial and maximum buffer sizes
- trimming for Windows-style line endings
- Memory-bounded long-line handling via `ErrTokenTooLong`
- Race-tested

## Installation

```bash
go get github.com/monitoring-forge/linebuf
```

## Usage

### Callback API

```go
package main

import (
    "os"

    "github.com/monitoring-forge/linebuf"
)

func main() {
    err := linebuf.Scan(os.Stdin, func(line []byte) error {
        // process line
        return nil
    })
    if err != nil {
        // handle error
    }
}
```

### Iterator API

```go
scanner := linebuf.New()
for line := range scanner.Iter(r) {
    // process line
}
if err := scanner.IterErr(); err != nil {
    // handle error
}
```

### Options

```go
scanner := linebuf.New(
    linebuf.WithStartBufSize(4096), // initial buffer size
    linebuf.WithMaxBufSize(65536),  // maximum buffer size
)
```

If a line exceeds `MaxBufSize`, scanning stops with `linebuf.ErrTokenTooLong`.


## License

See [LICENSE](LICENSE).
