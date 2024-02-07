YIFY Subtitles client
=========

[![GoDoc](https://godoc.org/github.com/golang/gddo?status.svg)](https://pkg.go.dev/github.com/odwrtw/yifysubs)
[![Go Report Card](https://goreportcard.com/badge/github.com/odwrtw/yifysubs)](https://goreportcard.com/report/github.com/odwrtw/yifysubs)

## Example

```go
package main

import (
    "io"
    "log"
    "os"

    "github.com/odwrtw/yifysubs"
)

func main() {
    // Create a client.
    client := yifysubs.NewDefault()

    imdbID := "tt3758542"

    // Search subtitles.
    subtitles, err := client.SearchByLang(imdbID, "English")
    if err != nil {
        log.Fatalf("Failed to get subtitles: %s", err)
        return
    }

    log.Printf("Found %d subtitles for movie with IMDB ID %s", len(subtitles), imdbID)

    // There will always be a first subtitles, if no subtitles where to be
    // found, the search function would return an error.
    firstSub := subtitles[0]

    path := "/tmp/" + imdbID + ".srt"
    file, err := os.Create(path)
    if err != nil {
        log.Fatalf("Failed to create file: %s", err)
        return
    }
    defer file.Close()

    if _, err := io.Copy(file, firstSub); err != nil {
        log.Fatalf("Failed to copy file: %s", err)
        return
    }

    log.Printf("Subtitle written to %s", path)
}
```
