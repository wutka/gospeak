package main

import (
	"flag"
	"fmt"
	"github.com/wutka/gospeak"
)

func main() {
	verboseFlag := flag.Bool("v", false, "Include diagnostic trace")
	quietFlag := flag.Bool("q", false, "Don't output speech")
	skipImportsFlag := flag.Bool("noimports", false, "Don't read imports")
	functionNameFlag := flag.String("func", "", "Read only specified function")
	outputFlag := flag.String("o", "", "Save speech to file")
	startFlag := flag.Int("start", -1, "Start at line")
	endFlag := flag.Int("end", -1, "End at line (inclusive)")

	flag.Parse()

	speaker := gospeak.MakeGoSpeaker(*quietFlag, *verboseFlag, *skipImportsFlag, *outputFlag)
	if *startFlag >= 0 && *endFlag >= 0 {
		if *endFlag < *startFlag {
			fmt.Printf("End line (%d) cannot be before start line (%d)\n", *endFlag, *startFlag)
			return
		}
		speaker.SetRange(*startFlag, *endFlag)

	}

	for _, filename := range flag.Args() {
		if *functionNameFlag == "" {
			speaker.SpeakGoFile(filename)
		} else {
			speaker.SpeakGoFunction(filename, *functionNameFlag)
		}
	}
}
