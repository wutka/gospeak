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

	speaker := gospeak.MakeGoSpeaker(*quietFlag, *verboseFlag, *skipImportsFlag, *outputFlag)

	for _, filename := range flag.Args() {
		if *functionNameFlag == "" {
			speaker.SpeakGoFile(filename)
		} else {
			speaker.SpeakGoFunction(filename, *functionNameFlag)
		}
	}
}
