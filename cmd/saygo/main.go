package main

import (
	"flag"
	"go-to-speech/pkg"
)

func main() {
	quietFlag := flag.Bool("q", false, "Don't output speech")
	skipImportsFlag := flag.Bool("noimports", false, "Don't read imports")
	functionNameFlag := flag.String("func", "", "Read only specified function")
	outputFlag := flag.String("o", "", "Save speech to file")

	flag.Parse()

	pkg.ShutUp = *quietFlag
	pkg.SkipImports = *skipImportsFlag
	pkg.TargetFunction = *functionNameFlag
	pkg.SayOut = *outputFlag

	for _, filename := range flag.Args() {
		pkg.SpeakGoFile(filename)
	}
}
