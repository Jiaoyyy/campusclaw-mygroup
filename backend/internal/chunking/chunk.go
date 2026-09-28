package chunking

import (
	"errors"
	"regexp"
	"strings"
)

type Options struct {
	Strategy           string `json:"strategy"`
	MaxChars           int    `json:"max_chars"`
	OverlapPercent     int    `json:"overlap_percent"`
	Separator          string `json:"separator"`
	RemoveURLs         bool   `json:"remove_urls"`
	RemoveEmails       bool   `json:"remove_emails"`
	CollapseWhitespace bool   `json:"collapse_whitespace"`
}

type Chunk struct {
	Index int
	Start int // Unicode rune offset in the text after optional preprocessing.
	End   int // Exclusive Unicode rune offset.
	Text  string
}

var (
	urlPattern   = regexp.MustCompile(`https?://[^\s]+`)
	emailPattern = regexp.MustCompile(`[[:alnum:]._%+-]+@[[:alnum:].-]+\.[[:alpha:]]{2,}`)
	spacePattern = regexp.MustCompile(`\s+`)
)

func (o Options) Normalized() (Options, error) {
	if o.Strategy == "" {
		o.Strategy = "auto"
	}
	switch o.Strategy {
	case "auto", "hierarchy":
		return Options{Strategy: o.Strategy, MaxChars: 800, OverlapPercent: 10}, nil
	case "custom":
		if o.MaxChars < 100 || o.MaxChars > 2000 || o.OverlapPercent < 0 || o.OverlapPercent > 50 {
			return Options{}, errors.New("custom chunk size must be 100-2000 and overlap 0-50 percent")
		}
		if o.Separator != "newline" && o.Separator != "blankline" && o.Separator != "period" {
			return Options{}, errors.New("custom separator must be newline, blankline or period")
		}
		return o, nil
	default:
		return Options{}, errors.New("strategy must be auto, custom or hierarchy")
	}
}

func Split(original string, requested Options) ([]Chunk, string, error) {
	o, err := requested.Normalized()
	if err != nil {
		return nil, "", err
	}
	processed := original
	if o.Strategy == "custom" {
		if o.RemoveURLs {
			processed = urlPattern.ReplaceAllString(processed, "")
		}
		if o.RemoveEmails {
			processed = emailPattern.ReplaceAllString(processed, "")
		}
		if o.CollapseWhitespace {
			processed = spacePattern.ReplaceAllString(processed, " ")
		}
	}
	runes := []rune(processed)
	if len(strings.TrimSpace(processed)) == 0 {
		return nil, processed, errors.New("no text remains after preprocessing")
	}
	var chunks []Chunk
	if o.Strategy == "hierarchy" {
		for _, section := range sections(runes) {
			chunks = append(chunks, windows(runes, section[0], section[1], 800, 80, "auto")...)
		}
	} else {
		overlap := o.MaxChars * o.OverlapPercent / 100
		chunks = windows(runes, 0, len(runes), o.MaxChars, overlap, o.Separator)
	}
	for i := range chunks {
		chunks[i].Index = i + 1
	}
	return chunks, processed, nil
}

func windows(runes []rune, from, to, maxChars, overlap int, separator string) []Chunk {
	if from >= to {
		return nil
	}
	var chunks []Chunk
	for start := from; start < to; {
		end := min(start+maxChars, to)
		if end < to {
			floor := start + maxChars/2
			for i := end - 1; i >= floor; i-- {
				if boundary(runes, i, separator) {
					end = i + 1
					break
				}
			}
		}
		chunks = append(chunks, Chunk{Start: start, End: end, Text: string(runes[start:end])})
		if end == to {
			break
		}
		start = max(start+1, end-overlap)
	}
	return chunks
}

func boundary(runes []rune, i int, separator string) bool {
	switch separator {
	case "newline":
		return runes[i] == '\n'
	case "blankline":
		return runes[i] == '\n' && i > 0 && runes[i-1] == '\n'
	case "period":
		return runes[i] == '。'
	default:
		return runes[i] == '。' || runes[i] == '\n'
	}
}

// sections returns heading-delimited rune ranges and preserves each heading.
func sections(runes []rune) [][2]int {
	starts := []int{0}
	for i := 0; i < len(runes); i++ {
		if i > 0 && runes[i-1] != '\n' {
			continue
		}
		j := i
		for j < len(runes) && j-i < 3 && runes[j] == '#' {
			j++
		}
		if j > i && j < len(runes) && runes[j] == ' ' && i > 0 {
			starts = append(starts, i)
		}
	}
	var out [][2]int
	for i, start := range starts {
		end := len(runes)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		if start < end {
			out = append(out, [2]int{start, end})
		}
	}
	return out
}
