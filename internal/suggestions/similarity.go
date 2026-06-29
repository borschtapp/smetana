package suggestions

import (
	"strings"

	"borscht.app/smetana/internal/utils"
)

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			sub := prev[j-1]
			if ra[i-1] != rb[j-1] {
				sub++
			}
			ins := curr[j-1] + 1
			del := prev[j] + 1
			curr[j] = min3(sub, ins, del)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// nameSimilarity returns a 0..1 score between two normalized names. It scores both the
// raw forms and their singularized forms (so "tomatoes" ≈ "tomato") and returns the higher.
func nameSimilarity(a, b string) float64 {
	na, nb := utils.CreateTag(a), utils.CreateTag(b)
	return nameKeySimilarity(nameKey{na, singularize(na)}, nameKey{nb, singularize(nb)})
}

// nameKey holds the normalized and singularized forms of a name, precomputed once per
// entity so a scan comparing many entities against each other doesn't redo this per pair.
type nameKey struct {
	norm string
	sing string
}

func buildNameKey(name string) nameKey {
	n := utils.CreateTag(name)
	return nameKey{norm: n, sing: singularize(n)}
}

func nameKeySimilarity(a, b nameKey) float64 {
	score := levenshteinScore(a.norm, b.norm)
	if a.sing != a.norm || b.sing != b.norm {
		if s := levenshteinScore(a.sing, b.sing); s > score {
			score = s
		}
	}
	return score
}

func levenshteinScore(na, nb string) float64 {
	if na == nb {
		return 1.0
	}
	maxLen := len([]rune(na))
	if l := len([]rune(nb)); l > maxLen {
		maxLen = l
	}
	if maxLen == 0 {
		return 1.0
	}
	dist := levenshtein(na, nb)
	score := 1.0 - float64(dist)/float64(maxLen)
	if score < 0 {
		return 0
	}
	return score
}

// singularize applies a light heuristic to strip common plural suffixes
func singularize(s string) string {
	r := []rune(s)
	n := len(r)
	switch {
	// English
	case n > 3 && strings.HasSuffix(s, "ies"):
		return string(r[:n-3]) + "y" // berries -> berry
	case n > 3 && strings.HasSuffix(s, "oes"):
		return string(r[:n-2]) // tomatoes -> tomato
	case n > 3 && (strings.HasSuffix(s, "ses") || strings.HasSuffix(s, "xes") || strings.HasSuffix(s, "zes") ||
		strings.HasSuffix(s, "ches") || strings.HasSuffix(s, "shes")):
		return string(r[:n-2]) // boxes -> box
	// German: weak/feminine nouns pluralize by appending "-n"/"-en" (Tomate -> Tomaten,
	// Zwiebel -> Zwiebeln, Kartoffel -> Kartoffeln). Length guard is higher than the other
	// rules since short words ending in "n" (e.g. "cumin") are common outside German.
	case n > 5 && strings.HasSuffix(s, "n"):
		return string(r[:n-1])
	// French: nouns ending -eau/-eu/-ou pluralize with "-x" instead of "-s"
	// (poireau -> poireaux, chou -> choux).
	case n > 4 && (strings.HasSuffix(s, "eaux") || strings.HasSuffix(s, "eux") || strings.HasSuffix(s, "oux")):
		return string(r[:n-1])
	// Italian: plurals shift the final vowel instead of appending a suffix
	// (pomodoro -> pomodori, cipolla -> cipolle). Words ending in -co/-go/-ca/-ga insert
	// an "h" before the plural's front vowel to keep the hard consonant sound, so these
	// four digraph cases must be checked before the plain vowel-swap rules below
	// (zucca -> zucche, not "zuccha"; fungo -> funghi, not "fungho").
	case n > 4 && strings.HasSuffix(s, "chi"):
		return string(r[:n-3]) + "co" // pacchi -> pacco
	case n > 4 && strings.HasSuffix(s, "ghi"):
		return string(r[:n-3]) + "go" // funghi -> fungo
	case n > 4 && strings.HasSuffix(s, "che"):
		return string(r[:n-3]) + "ca" // zucche -> zucca
	case n > 4 && strings.HasSuffix(s, "ghe"):
		return string(r[:n-3]) + "ga" // streghe -> strega
	case n > 4 && strings.HasSuffix(s, "i"):
		return string(r[:n-1]) + "o" // pomodori -> pomodoro
	case n > 4 && strings.HasSuffix(s, "e"):
		return string(r[:n-1]) + "a" // cipolle -> cipolla
	// English/French/Spanish: plain "-s" (onions -> onion, oignons -> oignon, tomates -> tomate).
	case n > 3 && strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss"):
		return string(r[:n-1])
	// Ukrainian: masculine nouns with a "fleeting vowel" drop the "о" before "-к" in
	// oblique/plural forms (огірок -> огірки, кабачок -> кабачки), so the plural suffix
	// "-ки" maps back to singular "-ок", not a plain strip. Checked before the plainer
	// "-и"/"-і" cases below since it's the more specific pattern.
	case n > 4 && strings.HasSuffix(s, "ки"):
		return string(r[:n-2]) + "ок"
	// Ukrainian: feminine nouns ending in "-я" pluralize to "-і" (цибуля -> цибулі,
	// вишня -> вишні), so "-я" must be restored rather than just stripping "-і".
	case n > 3 && strings.HasSuffix(s, "і"):
		return string(r[:n-1]) + "я"
	// Ukrainian: regular masculine plurals just append "-и" to a consonant stem
	// (помідор -> помідори, гриб -> гриби, банан -> банани).
	case n > 3 && strings.HasSuffix(s, "и"):
		return string(r[:n-1])
	default:
		return s
	}
}
