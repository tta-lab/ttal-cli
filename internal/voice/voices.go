package voice

import (
	"fmt"
	"os"
	"text/tabwriter"
)

const DefaultVoice = "af_heart"

type VoiceInfo struct {
	ID     string
	Gender string
	Accent string
	Note   string
}

var Voices = []VoiceInfo{
	// American English (Female)
	{"af_heart", "Female", "American", "Default, warm"},
	{"af_alloy", "Female", "American", ""},
	{"af_bella", "Female", "American", "Youthful, soft"},
	{"af_jessica", "Female", "American", ""},
	{"af_kore", "Female", "American", ""},
	{"af_nicole", "Female", "American", ""},
	{"af_nova", "Female", "American", "Professional"},
	{"af_river", "Female", "American", ""},
	{"af_sarah", "Female", "American", "Calm, composed"},
	{"af_sky", "Female", "American", "Bright, energetic"},

	// American English (Male)
	{"am_adam", "Male", "American", "Deep"},
	{"am_echo", "Male", "American", ""},
	{"am_eric", "Male", "American", ""},
	{"am_fenrir", "Male", "American", ""},
	{"am_liam", "Male", "American", ""},
	{"am_michael", "Male", "American", ""},
	{"am_onyx", "Male", "American", ""},
	{"am_puck", "Male", "American", ""},
	{"am_santa", "Male", "American", ""},

	// British English (Female)
	{"bf_alice", "Female", "British", ""},
	{"bf_emma", "Female", "British", "Elegant"},
	{"bf_lily", "Female", "British", ""},

	// British English (Male)
	{"bm_daniel", "Male", "British", ""},
	{"bm_fable", "Male", "British", ""},
	{"bm_george", "Male", "British", ""},
	{"bm_lewis", "Male", "British", ""},
}

// IsValidVoice checks if a voice ID is recognized.
func IsValidVoice(id string) bool {
	for _, v := range Voices {
		if v.ID == id {
			return true
		}
	}
	return false
}

// PrintVoiceList prints the available voices in a table.
func PrintVoiceList() {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tGENDER\tACCENT\tNOTE")
	for _, v := range Voices {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", v.ID, v.Gender, v.Accent, v.Note)
	}
	_ = w.Flush()
}
