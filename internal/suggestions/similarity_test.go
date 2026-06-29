package suggestions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSingularize(t *testing.T) {
	cases := map[string]string{
		// English
		"tomatoes": "tomato",
		"berries":  "berry",
		"onions":   "onion",
		"boxes":    "box",
		"gas":      "gas", // short word, left untouched
		"oz":       "oz",  // short word, left untouched
		"tomato":   "tomato",
		// German
		"tomaten":    "tomate",
		"zwiebeln":   "zwiebel",
		"kartoffeln": "kartoffel",
		"karotten":   "karotte",
		"birnen":     "birne",
		"grain":      "grain", // not German plural, too short for the -n rule
		// French
		"carottes": "carotte",
		"oignons":  "oignon",
		"poireaux": "poireau",
		"choux":    "chou",
		// Italian
		"pomodori": "pomodoro",
		"carciofi": "carciofo",
		"cipolle":  "cipolla",
		"zucche":   "zucca", // digraph: che -> ca, not "zuccha"
		"funghi":   "fungo", // digraph: ghi -> go, not "fungho"
		// Ukrainian
		"огірки":   "огірок", // fleeting vowel: ки -> ок, not "огірки"[:-1]
		"кабачки":  "кабачок",
		"цибулі":   "цибуля", // -я restored, not just stripped
		"вишні":    "вишня",
		"помідори": "помідор",
		"гриби":    "гриб",
		"банани":   "банан",
	}
	for input, want := range cases {
		assert.Equal(t, want, singularize(input), "singularize(%q)", input)
	}
}

func TestNameSimilarity_ExactMatch(t *testing.T) {
	assert.Equal(t, 1.0, nameSimilarity("Tomato", "tomato"))
}

func TestNameSimilarity_PluralMatchesSingular(t *testing.T) {
	// Without singularization these differ by 2 characters ("tomatoes" vs "tomato"),
	// which would previously score below most merge thresholds.
	assert.Equal(t, 1.0, nameSimilarity("Tomatoes", "Tomato"))
}

func TestNameSimilarity_GermanPluralMatchesSingular(t *testing.T) {
	assert.Equal(t, 1.0, nameSimilarity("Zwiebeln", "Zwiebel"))
}

func TestNameSimilarity_FrenchPluralMatchesSingular(t *testing.T) {
	assert.Equal(t, 1.0, nameSimilarity("Poireaux", "Poireau"))
}

func TestNameSimilarity_ItalianPluralMatchesSingular(t *testing.T) {
	assert.Equal(t, 1.0, nameSimilarity("Pomodori", "Pomodoro"))
}

func TestNameSimilarity_UkrainianPluralMatchesSingular(t *testing.T) {
	assert.Equal(t, 1.0, nameSimilarity("Огірки", "Огірок"))
	assert.Equal(t, 1.0, nameSimilarity("Цибулі", "Цибуля"))
}

func TestNameSimilarity_UnrelatedWords(t *testing.T) {
	score := nameSimilarity("Carrot", "Broccoli")
	assert.Less(t, score, 0.5)
}

func TestNameSimilarity_ShortDifferentWordsScoreLow(t *testing.T) {
	// These are the false-positive traps a naive distance metric falls into: short words
	// that share most of their letters but mean completely different things. Verifies the
	// added multilingual singularize rules don't inadvertently inflate these.
	cases := [][2]string{
		{"tsp", "tbsp"},
		{"salt", "malt"},
		{"basil", "basic"},
		{"pint", "print"},
		{"clove", "glove"},
		{"gram", "grain"},
	}
	for _, c := range cases {
		score := nameSimilarity(c[0], c[1])
		assert.Less(t, score, 0.85, "nameSimilarity(%q, %q)", c[0], c[1])
	}
}
