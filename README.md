YIFY Subtitles client
=========

[![GoDoc](https://godoc.org/github.com/golang/gddo?status.svg)](http://godoc.org/github.com/odwrtw/yifysubs)
[![Go Report Card](https://goreportcard.com/badge/github.com/odwrtw/yifysubs)](https://goreportcard.com/report/github.com/odwrtw/yifysubs)

## Example

```go
package main

import (
	"io"
	"log"
	"os"

	"github.com/odwrtw/yifysubs"
	"github.com/kr/pretty"
)

func main() {
  // Create a client
  client := yifysubs.New("http://yifysubtitles.com")

  // Search subtitles
  subtitles, err := client.Search("tt0133093")
  if err != nil {
      panic(err)
  }

  // Search subtitles by lang
  subtitles, err = client.SearchByLang("tt0133093", "French")
  if err != nil {
      panic(err)
  }

  for _, subtitle := range subtitles {
    pretty.Println(subtitle)
    file, err := os.Create("/tmp/tt0133083.fr.srt")
    if err != nil {
        panic(err)
    }
    defer file.Close()
    defer subtitle.Close()

    if _, err := io.Copy(file, subtitle); err != nil {
        panic(err)
    }
  }

}
```
