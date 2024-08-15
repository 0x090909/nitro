package main

import (
	"github.com/0x090909/nitro/linters/koanf"
	"github.com/0x090909/nitro/linters/pointercheck"
	"github.com/0x090909/nitro/linters/rightshift"
	"github.com/0x090909/nitro/linters/structinit"
	"golang.org/x/tools/go/analysis/multichecker"
)

func main() {
	multichecker.Main(
		koanf.Analyzer,
		pointercheck.Analyzer,
		rightshift.Analyzer,
		structinit.Analyzer,
	)
}
