package yifysubs

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
)

const defaultEndpoint = "https://yifysubtitles.ch"

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
			downloadURL = se.Request.AbsoluteURL(downloadURL)

			// Strip the leading <span> tag and normalize <br /> variants
			linkClone := downloadSel.Clone()
			linkClone.Find("span").Remove()
			releasesStr, err := linkClone.Html()
			if err != nil {
				return
			}
			releasesStr = strings.ReplaceAll(releasesStr, "<br/>", "\n")
			releasesStr = strings.ReplaceAll(releasesStr, "<br />", "\n")
			releases := strings.Split(releasesStr, "\n")
			for i := range releases {
				releases[i] = strings.Trim(releases[i], "\n ")
			}

			mu.Lock()
			subtitles = append(subtitles, &Subtitle{
				Lang:     lang,
				URL:      downloadURL,
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
	URL      string
	Releases []string
	buffer   *bytes.Buffer
}

// Read implement the reader interface
func (s *Subtitle) Read(p []byte) (n int, err error) {
	if s.buffer == nil {
		if err := s.download(); err != nil {
			return 0, err
		}
	}

	if s.buffer == nil {
		return 0, ErrNoSubtitleData
	}

	return s.buffer.Read(p)
}

func (s *Subtitle) download() error {
	zipURL := strings.Replace(s.URL, "/subtitles/", "/subtitle/", 1) + ".zip"

	req, err := http.NewRequest(http.MethodGet, zipURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Referer", s.URL)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("yify: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return err
	}

	for _, f := range zr.File {
		if filepath.Ext(f.Name) != ".srt" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		data, err := io.ReadAll(rc)
		if err != nil {
			return err
		}

		s.buffer = bytes.NewBuffer(data)
		return nil
	}

	return ErrNoSubtitleData
}

// Close implement the closer interface
func (s Subtitle) Close() error {
	if s.buffer != nil {
		s.buffer = nil
	}

	return nil
}
