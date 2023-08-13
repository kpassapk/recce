package recce

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type recce struct {
	name     string
	host     string
	port     string
	group    string
	prefix   string
	dir      string
	tc       int
	assert   *assert.Assertions
	req      *http.Request
	res      *http.Response
	reqBytes []byte
	resBytes []byte
}

type Client interface {
	NewRequest(method, url string, body io.Reader)
	SendRequest(handler http.HandlerFunc) *http.Response
	Finish()
}

type RecceConfig struct {
	host   string
	port   string
	group  string
	dir    string
	prefix string
}

func defaultConfig() *RecceConfig {
	return &RecceConfig{
		host:   "http://localhost",
		port:   "8080",
		group:  "rest",
		dir:    "recordings",
		prefix: "rec",
	}
}

type RecceOption func(*RecceConfig)

// WithHost sets the host in the output `.rest` file. It does not affect the HTTP request itself.
func WithHost(host string) RecceOption {
	return func(c *RecceConfig) {
		c.host = host
	}
}

// WithPort sets the port in the output `.rest` file. It does not affect the HTTP request itself.
func WithPort(port string) RecceOption {
	return func(c *RecceConfig) {
		c.port = port
	}
}

// WithGroup sets the group in the output `.rest` file. The group is a path-like string, for example `create/task`,
// which will be used to create a directory structure in the output directory. (See `WithOutputDirectory`.) The
// default group is "rest"
func WithGroup(group string) RecceOption {
	return func(c *RecceConfig) {
		c.group = group
	}
}

// WithOutput sets the output directory for the `.rest` files. The default is `recordings`.
func WithOutputDirectory(dir string) RecceOption {
	return func(c *RecceConfig) {
		c.dir = dir
	}
}

// WithPrefix sets the prefix for the `.rest` files. The default is `rec`.
func WithPrefix(prefix string) RecceOption {
	return func(c *RecceConfig) {
		c.prefix = prefix
	}
}

// Start saves a .rest file for test case number tc, having a descriptive name. It returns a Client for sending the request.
func Start(tc int, name string, a *assert.Assertions, opts ...RecceOption) Client {
	cfg := defaultConfig()

	for _, opt := range opts {
		opt(cfg)
	}

	return &recce{
		name:   name,
		host:   cfg.host,
		port:   cfg.port,
		group:  cfg.group,
		prefix: cfg.prefix,
		dir:    cfg.dir,
		tc:     tc,
		assert: a,
	}
}

// Finish writes files to disk, failing the test if there is an error.
func (s *recce) Finish() {
	var err error
	err = s.createTestFile()
	s.assert.NoError(err)
	err = s.createResponseFile()
	s.assert.NoError(err)
	err = s.res.Body.Close()
	s.assert.NoError(err)
}

// NewRequest creates a new HTTP request with the specified method and body.
func (s *recce) NewRequest(method, url string, body io.Reader) {
	req, err := http.NewRequest(method, url, body)
	s.assert.NoError(err)
	s.req = req
}

func (s *recce) SetHeader(key, value string) {
	s.req.Header.Set(key, value)
}

func (s *recce) SendRequest(handler http.HandlerFunc) *http.Response {
	rr := httptest.NewRecorder()
	if s.req.Body != nil {
		err := s.saveRequestBody()
		if err != nil {
			s.assert.FailNow(err.Error())
		}
	}
	handler(rr, s.req)
	s.res = rr.Result()
	if rr.Result().Body != nil {
		err := s.saveResponseBody()
		if err != nil {
			s.assert.FailNow(err.Error())
		}
	}
	return s.res
}

// prettyPrintRequest returns a string representation of the request in a format that is compatible with
// REST client and can be used to reproduce the request.
func (s *recce) prettyPrintRequest() (string, error) {
	var buf bytes.Buffer

	buf.WriteString(`// Automatically generated. Do not edit.
// To reproduce this request, install REST client extension for VS Code or Goland and run the "Send Request" command.
// File generated at ` + time.Now().Format("02 January 2006 15:04:05") + `
// ------------------------------------------------------------

`)
	buf.WriteString("// SCENARIO  " + strconv.Itoa(s.tc) + " (" + s.group + ")\n")
	buf.WriteString("// Name: " + s.name + "\n")

	// Print method, URL, and protocol
	buf.WriteString(fmt.Sprintf("%s %s:%s%s %s\n", s.req.Method, s.host, s.port, s.req.URL, s.req.Proto))

	// Print headers
	for k, values := range s.req.Header {
		for _, v := range values {
			buf.WriteString(fmt.Sprintf("%s: %s\n", k, v))
		}
	}

	if s.req.Body != nil {
		// Print a blank line to separate headers from body
		buf.WriteString("\n")

		printableBody := s.tryPrettyPrinting(s.reqBytes)
		buf.Write(printableBody)
	}

	return buf.String(), nil
}

// tryPrettyPrinting attempts to pretty print the body if it is JSON. If it fails, it returns the body as is.
func (s *recce) tryPrettyPrinting(body []byte) []byte {
	var printableBody []byte
	if s.isContentTypeJSON() {
		var pp bytes.Buffer
		err := json.Indent(&pp, body, "", "  ")
		if err != nil {
			// Silently ignore errors and print the body as is
			printableBody = body
		} else {
			printableBody = pp.Bytes()
		}
	} else {
		printableBody = body
	}
	return printableBody
}

func (s *recce) createTestFile() error {
	fullPath := s.fullPath()

	// Ensure the directories exist
	err := os.MkdirAll(fullPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("error creating directories: %v", err)
	}

	// Create the file
	fileName := fmt.Sprintf("sc%d.rest", s.tc)
	file, err := os.Create(filepath.Join(fullPath, fileName))
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer file.Close()

	// Write file contents
	content, err := s.prettyPrintRequest()
	if err != nil {
		return errors.Wrap(err, "error pretty printing request")
	}
	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}

	return nil
}

func (s *recce) prettyPrintResponse() (string, error) {
	var buf bytes.Buffer

	buf.WriteString(`// Automatically generated. Do not edit.
// File generated at ` + time.Now().Format("02 January 2006 15:04:05") + `
// ------------------------------------------------------------
`)

	// Write the request method, URL, and protocol to the buffer
	_, err := fmt.Fprintf(&buf, "%s %s %s\n\n", s.req.Method, s.req.URL.String(), s.req.Proto)
	if err != nil {
		return "", err
	}

	// Write the response status line to the buffer
	_, err = fmt.Fprintf(&buf, "%s %s\n", s.res.Proto, s.res.Status)
	if err != nil {
		return "", err
	}

	// Write each header to the buffer
	for key, values := range s.res.Header {
		for _, value := range values {
			_, err = fmt.Fprintf(&buf, "%s: %s\n", key, value)
			if err != nil {
				return "", err
			}
		}
	}

	// Write a blank line to separate headers from the body
	_, err = buf.WriteString("\n")
	if err != nil {
		return "", err
	}

	if s.res.Body != nil {
		b, err := io.ReadAll(s.res.Body)
		if err != nil {
			return "", err
		}
		_, err = buf.Write(s.tryPrettyPrinting(b))
		if err != nil {
			return "", err
		}
	}

	return buf.String(), nil
}

func (s *recce) createResponseFile() error {
	fullPath := s.fullPath()

	// Ensure the directories exist
	err := os.MkdirAll(fullPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("error creating directories: %v", err)
	}

	// Create the file
	fileName := fmt.Sprintf("sc%d.resp", s.tc)
	file, err := os.Create(filepath.Join(fullPath, fileName))
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer file.Close()

	// Write file contents
	content, err := s.prettyPrintResponse()
	if err != nil {
		return errors.Wrap(err, "error pretty printing response")
	}
	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}

	return nil
}

func (s *recce) fullPath() string {
	directories := strings.Split(s.group, "/")

	// Build the full path starting from "examples"
	return filepath.Join(s.dir, filepath.Join(directories...))
}

// SaveRequestBody saves the request body into a byte slice
// and restores the body so it can be used normally afterward.
func (s *recce) saveRequestBody() error {
	// Check if the request has a body
	if s.req.Body == nil {
		return errors.New("request has no body")
	}

	// Read the request body
	var err error
	s.reqBytes, err = io.ReadAll(s.req.Body)
	if err != nil {
		return err
	}

	// Restore the request body to its original state
	s.req.Body = io.NopCloser(bytes.NewBuffer(s.reqBytes))

	return nil
}

func (s *recce) isContentType(ct string) bool {
	// Get the Content-Type header from the request
	contentType := s.req.Header.Get("Content-Type")

	// Split the header on ";" to consider only the primary type
	mimeType := strings.Split(contentType, ";")[0]

	// Trim any whitespace and check against "application/json"
	return strings.TrimSpace(mimeType) == ct
}

func (s *recce) isContentTypeJSON() bool {
	return s.isContentType("application/json")
}

func (s *recce) saveResponseBody() error {
	// Check if the response has a body
	if s.res.Body == nil {
		return errors.New("response has no body")
	}

	// Read the response body
	var err error
	s.resBytes, err = io.ReadAll(s.res.Body)
	if err != nil {
		return err
	}

	// Restore the request body to its original state
	s.res.Body = io.NopCloser(bytes.NewBuffer(s.resBytes))

	return nil
}
