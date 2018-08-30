package main

import (
	"flag"
	"github.com/wutka/gospeak"
)

func main() {
	verboseFlag := flag.Bool("v", false, "Include diagnostic trace")
	quietFlag := flag.Bool("q", false, "Don't output speech")
	skipImportsFlag := flag.Bool("noimports", false, "Don't read imports")
	functionNameFlag := flag.String("func", "", "Read only specified function")
	outputFlag := flag.String("o", "", "Save speech to file")

	flag.Parse()

	gospeak.ShutUp = *quietFlag
	gospeak.SkipImports = *skipImportsFlag
	gospeak.TargetFunction = *functionNameFlag
	gospeak.SayOut = *outputFlag
	gospeak.VerboseOutput = *verboseFlag

	for _, filename := range flag.Args() {
		gospeak.SpeakGoFile(filename)
	}
}
