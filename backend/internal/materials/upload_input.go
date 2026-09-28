package materials

import (
	"errors"
	"io"
	"mime/multipart"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"campusclaw/backend/internal/chunking"
)

type uploadInput struct {
	filename  string
	extension string
	content   []byte
	options   chunking.Options
}

func readUpload(reader *multipart.Reader, maxBytes int64) (uploadInput, error) {
	var input uploadInput
	fields := make(map[string]string)
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return uploadInput{}, errors.New("invalid multipart form")
		}
		name := part.FormName()
		if name == "file" {
			if input.content != nil || part.FileName() == "" {
				return uploadInput{}, errors.New("exactly one file required")
			}
			input.filename = filepath.Base(strings.ReplaceAll(part.FileName(), "\\", "/"))
			input.extension = strings.ToLower(filepath.Ext(input.filename))
			if len(input.filename) > 255 || (input.extension != ".txt" && input.extension != ".md") {
				return uploadInput{}, errors.New("only .txt and .md files are supported")
			}
			input.content, err = io.ReadAll(io.LimitReader(part, maxBytes+1))
			if err != nil {
				return uploadInput{}, errors.New("file read failed")
			}
			if int64(len(input.content)) > maxBytes {
				return uploadInput{}, errTooLarge
			}
			continue
		}
		if _, exists := fields[name]; exists {
			return uploadInput{}, errors.New("duplicate upload field")
		}
		switch name {
		case "strategy", "max_chars", "overlap_percent", "separator", "remove_urls", "remove_emails", "collapse_whitespace":
		default:
			return uploadInput{}, errors.New("unknown upload field")
		}
		data, err := io.ReadAll(io.LimitReader(part, 1025))
		if err != nil || len(data) > 1024 {
			return uploadInput{}, errors.New("upload field is too long")
		}
		fields[name] = string(data)
	}
	if input.content == nil {
		return uploadInput{}, errors.New("file required")
	}
	if !utf8.Valid(input.content) || strings.TrimSpace(string(input.content)) == "" {
		return uploadInput{}, errors.New("non-empty UTF-8 text required")
	}
	options, err := fieldsToOptions(fields)
	if err != nil {
		return uploadInput{}, err
	}
	input.options = options
	return input, nil
}

var errTooLarge = errors.New("file too large")

func fieldsToOptions(fields map[string]string) (chunking.Options, error) {
	o := chunking.Options{Strategy: fields["strategy"], Separator: fields["separator"]}
	for _, field := range []struct {
		name   string
		target *int
	}{{"max_chars", &o.MaxChars}, {"overlap_percent", &o.OverlapPercent}} {
		if value := fields[field.name]; value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return o, errors.New("invalid chunking number")
			}
			*field.target = parsed
		}
	}
	for _, field := range []struct {
		name   string
		target *bool
	}{{"remove_urls", &o.RemoveURLs}, {"remove_emails", &o.RemoveEmails}, {"collapse_whitespace", &o.CollapseWhitespace}} {
		if value := fields[field.name]; value != "" {
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return o, errors.New("invalid chunking flag")
			}
			*field.target = parsed
		}
	}
	return o.Normalized()
}
