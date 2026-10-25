package memory

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

type lexicalDocument struct {
	ID             string
	Tokens         []string
	TermCounts     map[string]int
	Length         int
	NormalizedText string
}

type lexicalIndex struct {
	Documents []lexicalDocument
	DocFreq   map[string]int
	AvgLength float64
}

type lexicalHit struct {
	ID    string
	Score float64
}

func newLexicalIndex(scenes []SceneRecord) *lexicalIndex {
	index := &lexicalIndex{
		Documents: make([]lexicalDocument, 0, len(scenes)),
		DocFreq:   map[string]int{},
	}
	totalLength := 0
	for _, scene := range scenes {
		searchText := strings.Join([]string{
			scene.ChapterTitle,
			scene.Text,
			strings.Join(scene.Characters, " "),
			strings.Join(scene.Locations, " "),
			strings.Join(scene.Items, " "),
			strings.Join(scene.Organizations, " "),
			strings.Join(scene.Storylines, " "),
			strings.Join(scene.RecapSignals, " "),
		}, " ")
		tokens := Tokenize(searchText)
		counts := make(map[string]int, len(tokens))
		for _, token := range tokens {
			counts[token]++
		}
		for token := range counts {
			index.DocFreq[token]++
		}
		totalLength += len(tokens)
		index.Documents = append(index.Documents, lexicalDocument{
			ID:             scene.ID,
			Tokens:         tokens,
			TermCounts:     counts,
			Length:         len(tokens),
			NormalizedText: strings.ToLower(searchText),
		})
	}
	if len(index.Documents) > 0 {
		index.AvgLength = float64(totalLength) / float64(len(index.Documents))
	}
	return index
}

func (index *lexicalIndex) Search(query string, limit int) []lexicalHit {
	if index == nil || len(index.Documents) == 0 {
		return nil
	}
	queryTerms := Tokenize(query)
	if len(queryTerms) == 0 {
		return nil
	}
	queryCounts := make(map[string]int, len(queryTerms))
	for _, token := range queryTerms {
		queryCounts[token]++
	}
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	const (
		k1 = 1.2
		b  = 0.75
	)
	hits := make([]lexicalHit, 0)
	for _, document := range index.Documents {
		score := 0.0
		for token, queryFrequency := range queryCounts {
			termFrequency := document.TermCounts[token]
			if termFrequency == 0 {
				continue
			}
			documentFrequency := index.DocFreq[token]
			if documentFrequency == 0 {
				continue
			}
			idf := math.Log(1 + (float64(len(index.Documents)-documentFrequency)+0.5)/(float64(documentFrequency)+0.5))
			normalizer := float64(termFrequency) + k1*(1-b+b*float64(document.Length)/math.Max(index.AvgLength, 1))
			score += idf * (float64(termFrequency) * (k1 + 1) / normalizer) * float64(queryFrequency)
		}
		if normalizedQuery != "" && len([]rune(normalizedQuery)) >= 2 && strings.Contains(document.NormalizedText, normalizedQuery) {
			score += 2.0
		}
		if score > 0 {
			hits = append(hits, lexicalHit{ID: document.ID, Score: score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].ID < hits[j].ID
		}
		return hits[i].Score > hits[j].Score
	})
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

// Tokenize produces lowercase Latin/word tokens plus overlapping CJK
// uni/bi/trigrams. The n-grams make BM25 useful for Chinese without pulling a
// dictionary-based segmenter into the CLI runtime.
func Tokenize(text string) []string {
	text = strings.TrimSpace(strings.ToLower(text))
	if text == "" {
		return nil
	}
	tokens := make([]string, 0, len([]rune(text))/2)
	word := make([]rune, 0)
	cjk := make([]rune, 0)
	flushWord := func() {
		if len(word) == 0 {
			return
		}
		value := string(word)
		if !lexicalStopWords[value] && len([]rune(value)) > 1 {
			tokens = append(tokens, value)
		}
		word = word[:0]
	}
	flushCJK := func() {
		if len(cjk) == 0 {
			return
		}
		tokens = append(tokens, cjkNGrams(cjk)...)
		cjk = cjk[:0]
	}
	for _, r := range text {
		switch {
		case isCJK(r):
			flushWord()
			cjk = append(cjk, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			flushCJK()
			word = append(word, r)
		default:
			flushWord()
			flushCJK()
		}
	}
	flushWord()
	flushCJK()
	return tokens
}

func cjkNGrams(runes []rune) []string {
	if len(runes) == 0 {
		return nil
	}
	if len(runes) == 1 {
		return []string{string(runes)}
	}
	out := make([]string, 0, len(runes)*3)
	for size := 1; size <= 3; size++ {
		if size > len(runes) {
			break
		}
		for start := 0; start+size <= len(runes); start++ {
			value := string(runes[start : start+size])
			if !lexicalStopWords[value] {
				out = append(out, value)
			}
		}
	}
	return out
}

func isCJK(r rune) bool {
	return unicode.In(r,
		unicode.Han,
		unicode.Hiragana,
		unicode.Katakana,
		unicode.Hangul,
	)
}

var lexicalStopWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "by": true, "for": true, "from": true, "in": true, "is": true,
	"it": true, "of": true, "on": true, "or": true, "that": true, "the": true,
	"this": true, "to": true, "was": true, "were": true, "with": true,
	"的": true, "了": true, "在": true, "是": true, "和": true, "与": true,
	"也": true, "就": true, "都": true, "而": true, "着": true, "把": true,
	"被": true, "让": true, "给": true, "从": true, "向": true, "到": true,
}
