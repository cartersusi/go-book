package main

import (
	"math"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

func TextQuality(text string) (float32, bool) {
	valid_utf8 := is_utf8(text)
	if !valid_utf8 {
		return 0, false
	}

	var score float32
	score += unwanted_patterns(text) * 0.25
	score += spam(text) * 0.25
	score += alphanumeric_ratio(text) * 0.4

	return score, true
}

func unwanted_patterns(text string) float32 {
	patterns := []string{
		`[\x00-\x1F]`,                // Control characters
		`[\x{fffd}\x{25a1}\x{2370}]`, // Replacement character, White square, APL FUNCTIONAL SYMBOL QUAD
		`�`,                          // Unicode replacement character
		`\s{8,}`,                     // 8 or more consecutive whitespace characters
		`[^\x00-\x7F]+`,              // Non-ASCII characters
		`[^\w\s]`,                    // Any character that is not a word character or whitespace
	}

	unwanted := len(patterns)
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(text) {
			unwanted--
		}
	}

	return (float32(unwanted) / float32(len(patterns)))
}

func spam(text string) float32 {
	num_spaces := strings.Count(text, " ")
	num_words := len(strings.Fields(text))
	if num_words == 0 || num_spaces == 0 {
		return 0
	}

	// TODO: maybe scale log this
	return float32(1 - (float32(math.Abs(float64(num_spaces-num_words))) / float32(num_spaces+num_words)))
}

func is_utf8(s string) bool {
	if len(s) == 0 {
		return false
	}

	return utf8.Valid([]byte(s))
}

func alphanumeric_ratio(s string) float32 {
	if len(s) == 0 {
		return 0
	}

	is_alpha_numeric := 0
	for _, r := range s {
		if unicode.IsLetter(r) && unicode.IsNumber(r) {
			is_alpha_numeric++
		}
	}
	return (float32(is_alpha_numeric) / float32(len(s)))
}

func is_alpha_numeric(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}

	return true
}
