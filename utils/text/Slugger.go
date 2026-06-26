package text

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

func Slugify(input string) string {
	return lowerCase(hyphenate(removeSpaces(normalize(input))))
}

func normalize(input string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, input)

	return result
}

func lowerCase(input string) string {
	return strings.ToLower(input)
}

func removeSpaces(input string) string {
	re := regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(input, " ")
}

func hyphenate(input string) string {
	re1 := regexp.MustCompilePOSIX("[^a-zA-Z0-9]+")
	result := re1.ReplaceAllString(input, "-")
	re2 := regexp.MustCompilePOSIX(`-+`)
	result = re2.ReplaceAllString(result, "-")

	return strings.Trim(result, "-")
}
