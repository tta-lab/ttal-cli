package cmd

import (
	"github.com/tta-lab/logos"
	"github.com/tta-lab/ttal-cli/internal/ask"
)

var (
	urlCommandDoc = ask.URLCommandDoc
	webCommandDoc = ask.WebCommandDoc
	rgCommandDoc  = ask.RGCommandDoc
	srcCommandDoc = ask.SrcCommandDoc
)

func networkCommands() []logos.CommandDoc { return ask.NetworkCommands() }
func allCommands() []logos.CommandDoc     { return ask.AllCommands() }
