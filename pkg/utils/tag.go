package utils

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	kUtils "github.com/borschtapp/krip/utils"
)

var transformer = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
var regex = regexp.MustCompile(`[^\p{L}\p{N} ]+`)

func clearNonAlphanumeric(str string) string {
	return regex.ReplaceAllString(str, "")
}

func replaceDiacritics(s string) string {
	result, _, _ := transform.String(transformer, s)
	return result
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func CreateTag(name string) string {
	if !isASCII(name) {
		name = replaceDiacritics(name)
	}

	return strings.ToLower(clearNonAlphanumeric(name))
}

func CreateHostnameTag(url string) string {
	return kUtils.Hostname(url)
}
