package yifysubs

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

// Endpoint of the API
const APIEndpoint = "http://api.yifysubtitles.com/subs"

// Subtitle download endpoint
const SubtitleEndpoint = "http://www.yifysubtitles.com"

// Errors
var (
	ErrNotSubtitleFound      = errors.New("yify: no subtitles found")
	ErrMissingSubtitleURL    = errors.New("yify: missing subtitle URL")
	ErrFileAlreadyDownloaded = errors.New("yify: file already downloaded")
	ErrFailedToUnzip         = errors.New("yify: failed to unzip archive")
)

// Response is the representation of a response from the YIFY subtitle API
type Response struct {
	SubtitlesCount int                              `json:"subtitles"`
	Subtitles      map[string]map[string][]Subtitle `json:"subs"`
}

// GetSubtitles search for the subtitles from an imdb id
func GetSubtitles(imdbID string) (map[string][]Subtitle, error) {
	URL := APIEndpoint + "/" + imdbID

	resp, err := http.Get(URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Unmarshal the result
	var response *Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	// No sub found
	if response.SubtitlesCount == 0 {
		return nil, ErrNotSubtitleFound
	}

	// Search for the subs of this movie only
	res, ok := response.Subtitles[imdbID]
	if !ok {
		return nil, ErrNotSubtitleFound
	}

	return res, nil
}

// Subtitle is the representation of a subtitle
type Subtitle struct {
	ID            int
	Rating        int
	URL           string
	zipPath       string
	zipFileReader io.ReadCloser
	zipReader     *zip.ReadCloser
}

// UnmarshalJSON implements the unmarshaler interface
func (s *Subtitle) UnmarshalJSON(data []byte) error {
	dataBytes := bytes.NewReader(data)
	var aux struct {
		ID     int    `json:"id"`
		Rating int    `json:"rating"`
		URL    string `json:"url"`
	}

	// Decode json into the aux struct
	if err := json.NewDecoder(dataBytes).Decode(&aux); err != nil {
		return err
	}

	// Save the unmarshaled data
	s.ID = aux.ID
	s.Rating = aux.Rating
	s.URL = SubtitleEndpoint + aux.URL

	return nil
}

// downloadZip downloads the zip file to a tmp directory
func (s *Subtitle) downloadZip() error {
	if s.URL == "" {
		return ErrMissingSubtitleURL
	}

	if s.zipFileReader != nil {
		return ErrFileAlreadyDownloaded
	}

	resp, err := http.Get(s.URL)
	if err != nil {
		return err
	}

	file, err := ioutil.TempFile(os.TempDir(), "yify")
	if err != nil {
		return err
	}
	s.zipPath = file.Name()

	defer resp.Body.Close()
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		log.Panic(err)
	}

	r, err := zip.OpenReader(file.Name())
	if err != nil {
		log.Panic(err)
	}

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}

		// Stop at the first file, the zip only contains one file
		s.zipFileReader = rc
		break
	}

	return nil
}

// Read implement the reader interface
func (s *Subtitle) Read(p []byte) (n int, err error) {
	if s.zipFileReader == nil {
		// Download the zip and get the file reader
		if err := s.downloadZip(); err != nil {
			return 0, err
		}

		if s.zipFileReader == nil {
			return 0, ErrFailedToUnzip
		}
	}

	return s.zipFileReader.Read(p)
}

// Close implement the closer interface
func (s *Subtitle) Close() error {
	if s.zipFileReader != nil {
		s.zipFileReader.Close()
	}

	if s.zipReader != nil {
		s.zipReader.Close()
	}

	os.Remove(s.zipPath)

	return nil
}
