package gospeak

import (
	"strings"
	"testing"
)

func TestHelloWorld(t *testing.T) {
	prog := `
package main

import "fmt"

func main() {
	fmt.Printf("Hello World!\n")
}`

	goSpeaker := goSpeaker{
		quiet:         true,
		verboseOutput: false,
		skipImports:   false,
	}

	goSpeaker.SpeakGoString(prog)

	speechCommands := stripNewlines(stripPause(goSpeaker.speechBuffer.String()))

	target := `package main
imports fumt
declarations
function main taking no parameters and returning no values
function body
fumt dot Printf of Hello World! backslash n
end function main `

	splits := splitCommands(speechCommands)
	targetSplits := splitCommands(stripNewlines(target))

	if len(targetSplits) != len(splits) {
		t.Errorf("Target has %d items, speech has %d\n",
			len(targetSplits), len(splits))
		t.Fail()
		return
	}

	for i := range targetSplits {
		if targetSplits[i] != splits[i] {
			t.Errorf("Target mismatch at target=%s speeck=%s\n",
				targetSplits[i], splits[i])
		}
	}
}

func TestTopLevelDeclaration(t *testing.T) {
	prog := `
package main

var foo int
`

	goSpeaker := goSpeaker{
		quiet:         true,
		verboseOutput: false,
		skipImports:   false,
	}

	goSpeaker.SpeakGoString(prog)

	speechCommands := stripNewlines(stripPause(goSpeaker.speechBuffer.String()))

	target := "var foo of type int"

	splits := splitCommands(speechCommands)
	targetSplits := splitCommands(stripNewlines(target))

	if !hasSubsequence(splits, targetSplits) {
		t.Errorf("Could not find subsequence: %s\n", target)
	}
}

func TestEmptyInterface(t *testing.T) {
	prog := `
package main

var foo interface{}
`

	goSpeaker := goSpeaker{
		quiet:         true,
		verboseOutput: false,
		skipImports:   false,
	}

	goSpeaker.SpeakGoString(prog)

	speechCommands := stripNewlines(stripPause(goSpeaker.speechBuffer.String()))

	target := "var foo of type empty interface"

	splits := splitCommands(speechCommands)
	targetSplits := splitCommands(stripNewlines(target))

	if !hasSubsequence(splits, targetSplits) {
		t.Errorf("Could not find subsequence: %s\n", target)
	}
}

func splitCommands(s string) []string {
	commands := []string{}
	splits := strings.Split(s, " ")
	for _, split := range splits {
		if split == "\"" {
			continue
		}
		if len(strings.TrimSpace(split)) == 0 {
			continue
		}
		commands = append(commands, split)
	}
	return commands
}

func stripPause(s string) string {
	return strings.Replace(s, "{pause}", " ", -1)
}

func stripNewlines(s string) string {
	return strings.Replace(s, "\n", " ", -1)
}

func hasSubsequence(commands, sub []string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(commands)-len(sub); i++ {
		if commands[i] == sub[0] {
			found := true
			for j := 1; j < len(sub); j++ {
				if commands[i+j] != sub[j] {
					found = false
					break
				}
			}
			if found {
				return true
			}
		}
	}
	return false
}
