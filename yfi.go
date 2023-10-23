package yifysubs

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jpillora/scraper/scraper"
	"github.com/mitchellh/mapstructure"
)

// Errors
var (
	ErrNoSubtitleFound = errors.New("yify: no subtitles found")
)

// Client represent a Client used to make Search
type Client struct {
	scraper  *scraper.Endpoint
	Endpoint string
}

// Subtitle represents a Subtitle
type Subtitle struct {
	Rating   int
	Lang     string
	Uploader string
	URL      string
	Title    string
	reader   io.ReadCloser
}

// New return a new Searcher
func New(endpoint string) *Client {
	e := &scraper.Endpoint{
		Name:   "yifysubtitles",
		Method: "GET",
		List:   "table.other-subs > tbody > tr",
		Headers: map[string]string{
			"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/57.0.2988.133 Safari/537.36",
		},
		Result: map[string]scraper.Extractors{
			"rating":   scraper.Extractors{scraper.MustExtractor("td.rating-cell"), scraper.MustExtractor("span")},
			"lang":     scraper.Extractors{scraper.MustExtractor("td.flag-cell"), scraper.MustExtractor("span.sub-lang")},
			"title":    scraper.Extractors{scraper.MustExtractor("td:nth-child(3)"), scraper.MustExtractor("a"), scraper.MustExtractor("/subtitle (.*)/")},
			"uploader": scraper.Extractors{scraper.MustExtractor("td.uploader-cell"), scraper.MustExtractor("a")},
			"url":      scraper.Extractors{scraper.MustExtractor("td.download-cell"), scraper.MustExtractor("a"), scraper.MustExtractor("@href")},
		},
		Debug: false,
	}

	return &Client{
		scraper:  e,
		Endpoint: endpoint,
	}
}

// Search will search Subtitles
func (c *Client) Search(imdbID string) ([]*Subtitle, error) {
	c.scraper.URL = c.Endpoint + "/movie-imdb/{{imdbId}}"

	vars := map[string]string{
		"imdbId": imdbID,
	}
	return c.parseSubtitle(vars)
}

// SearchByLang will search Subtitles with given language
// The result will be ordered, with the highest rated subtitle first
func (c *Client) SearchByLang(imdbID, lang string) ([]*Subtitle, error) {
	subtitles, err := c.Search(imdbID)
	if err != nil {
		return nil, err
	}

	return FilterByLang(subtitles, lang), nil
}

// FilterByLang will filter the subtitles by language
// The result will be ordered, with the highest rated subtitle first
func FilterByLang(subtitles []*Subtitle, language string) []*Subtitle {
	filterredSubtitles := []*Subtitle{}
	for _, s := range subtitles {
		if s.Lang == language {
			filterredSubtitles = append(filterredSubtitles, s)
		}
	}
	sort.Slice(filterredSubtitles, func(i, j int) bool { return filterredSubtitles[i].Rating > filterredSubtitles[j].Rating })

	return filterredSubtitles
}

// parseSubtitle takes a map of parameters, it will do the request and return
// the parsed Subtitles
func (c *Client) parseSubtitle(vars map[string]string) ([]*Subtitle, error) {
	// Parse the page
	res, err := c.scraper.Execute(vars)
	if err != nil {
		return nil, err
	}

	subtitles := []*Subtitle{}

	// Map the res to our structure
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           &subtitles,
	})
	if err != nil {
		return nil, err
	}

	if err = decoder.Decode(res); err != nil {
		return nil, err
	}

	if len(subtitles) == 0 {
		return nil, ErrNoSubtitleFound
	}

	// Add the endpoint in front of the URLs
	for _, s := range subtitles {
		s.URL = c.Endpoint + s.URL
	}

	return subtitles, nil
}

// DownloadZipURL returns the zip file URL of the subtitle
func (s Subtitle) DownloadZipURL() string {
	return fmt.Sprintf("%s.zip", strings.Replace(s.URL, "/subtitles/", "/subtitle/", -1))
}

func getReaderFromURL(url string) (io.ReadCloser, error) {
	// Download the zip file
	var httpClient = &http.Client{
		Timeout: time.Second * 10,
	}
	res, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got http error %q", res.StatusCode)
	}

	// Read all the body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body with error %s", err)
	}

	// Create a new zip Reader from a newly created bytes reader of the body
	// already read
	r, err := zip.NewReader(bytes.NewReader(body), res.ContentLength)
	if err != nil {
		return nil, err
	}

	for _, f := range r.File {
		if filepath.Ext(f.Name) != ".srt" {
			continue
		}
		return f.Open()
	}
	return nil, fmt.Errorf("empty zip subtitle")
}

// Read implement the reader interface
func (s *Subtitle) Read(p []byte) (n int, err error) {
	if s.reader == nil {
		// Download the zip and get the file reader
		r, err := getReaderFromURL(s.DownloadZipURL())
		if err != nil {
			return 0, err
		}
		s.reader = r
	}

	return s.reader.Read(p)
}

// Close implement the closer interface
func (s Subtitle) Close() error {
	if s.reader != nil {
		return s.reader.Close()
	}

	return nil
}
