package yifysubs

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
)

const defaultEndpoint = "https://yifysubtitles.me"

// Custom errors
var (
	ErrNoSubtitleFound = errors.New("yify: no subtitles found")
	ErrNoSubtitleData  = errors.New("yify: no subtitles data")
)

// Client represent a Client used to make Search
type Client struct {
	Endpoint string
}

// New return a new client
func New(endpoint string) *Client {
	return &Client{Endpoint: endpoint}
}

// NewDefault return a new client with a default endpoint
func NewDefault() *Client {
	return &Client{Endpoint: defaultEndpoint}
}

// Search will search Subtitles
func (c *Client) Search(imdbID string) ([]*Subtitle, error) {
	var mu sync.Mutex
	subtitles := []*Subtitle{}

	client := colly.NewCollector()
	extensions.RandomUserAgent(client)
	extensions.Referer(client)

	client.OnHTML("table.other-subs", func(e *colly.HTMLElement) {
		e.ForEach("tbody > tr", func(i int, se *colly.HTMLElement) {
			langSel := se.DOM.Find("td.flag-cell > span.sub-lang")
			lang, err := langSel.Html()
			if err != nil {
				return
			}

			downloadSel := se.DOM.Find("td:nth-child(3) > a")
			downloadURL, ok := downloadSel.Attr("href")
			if !ok {
				return
			}

			releasesStr, err := downloadSel.Html()
			if err != nil {
				return
			}
			releases := strings.Split(releasesStr, "<br/>")
			for i := range releases {
				releases[i] = strings.Trim(releases[i], "\n ")
			}

			mu.Lock()
			subtitles = append(subtitles, &Subtitle{
				Lang:     lang,
				url:      downloadURL,
				Releases: releases,
			})
			mu.Unlock()
		})
	})

	endpoint := c.Endpoint + "/movie-imdb/" + imdbID
	if err := client.Visit(endpoint); err != nil {
		return nil, err
	}
	client.Wait()

	return returnSubs(subtitles)
}

// SearchByLang searches Subtitles with given language
func (c *Client) SearchByLang(imdbID, lang string) ([]*Subtitle, error) {
	subtitles, err := c.Search(imdbID)
	if err != nil {
		return nil, err
	}

	return FilterByLang(subtitles, lang)
}

// FilterByLang will filter the subtitles by language
func FilterByLang(subtitles []*Subtitle, language string) ([]*Subtitle, error) {
	subs := []*Subtitle{}
	for _, s := range subtitles {
		if s.Lang == language {
			subs = append(subs, s)
		}
	}

	return returnSubs(subs)
}

func returnSubs(subs []*Subtitle) ([]*Subtitle, error) {
	if len(subs) == 0 {
		return nil, ErrNoSubtitleFound
	}

	return subs, nil
}

// Subtitle represents a Subtitle
type Subtitle struct {
	Lang     string
	url      string
	Releases []string
	buffer   *bytes.Buffer
}

// Read implement the reader interface
func (s *Subtitle) Read(p []byte) (n int, err error) {
	if s.buffer == nil {
		client := colly.NewCollector()
		extensions.RandomUserAgent(client)
		extensions.Referer(client)

		client.OnResponse(func(r *colly.Response) {
			if r.Headers.Get("Content-Type") != "application/zip" {
				return
			}

			lengthStr := r.Headers.Get("Content-Length")
			length, err := strconv.Atoi(lengthStr)
			if err != nil {
				return
			}

			// Create a new zip.Reader from the response body
			zr, err := zip.NewReader(bytes.NewReader(r.Body), int64(length))
			if err != nil {
				return
			}

			for _, f := range zr.File {
				if filepath.Ext(f.Name) != ".srt" {
					continue
				}

				file, err := f.Open()
				if err != nil {
					return
				}
				defer file.Close()

				data, err := io.ReadAll(file)
				if err != nil {
					return
				}

				s.buffer = bytes.NewBuffer(data)
				return
			}
		})

		client.OnHTML("a.download-subtitle", func(e *colly.HTMLElement) {
			urlEncoded := e.Attr("onclick")
			parts := strings.Split(urlEncoded, "'")
			if len(parts) != 3 {
				return
			}

			urlEncoded = parts[1]
			url, err := base64.StdEncoding.DecodeString(urlEncoded)
			if err != nil {
				return
			}
			client.Visit(string(url))
		})

		if err := client.Visit(s.url); err != nil {
			return 0, err
		}
		client.Wait()
	}

	if s.buffer == nil {
		return 0, ErrNoSubtitleData
	}

	return s.buffer.Read(p)
}

// Close implement the closer interface
func (s Subtitle) Close() error {
	if s.buffer != nil {
		s.buffer = nil
	}

	return nil
}
