YIFY Subtitles client
=========

[![GoDoc](https://godoc.org/github.com/golang/gddo?status.svg)](http://godoc.org/github.com/odwrtw/yifysubs)

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
	subs, err := yifysubs.GetSubtitles("tt1243974")
	if err != nil {
		log.Panic(err)
	}
	fr := subs["french"][0]

	file, err := os.Create("test.srt")
	if err != nil {
		log.Panic(err)
	}

	defer file.Close()
	defer fr.Close()

	_, err = io.Copy(file, &fr)
	if err != nil {
		log.Panic(err)
	}
}
```
