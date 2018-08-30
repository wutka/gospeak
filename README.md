# A Go-specific Text-to-Speech converter

This is a proof-of-concept program to do text-to-speech using the Go AST parser. It
attempts to do an English-language rendering of what the code is doing, rather than
just reading individual words and characters as a regular TTS engine would.

It currently relies on the Mac OSX "say" program, so it won't run on another platform
without reworking the speak function.

To run:

```
go get github.com/gospeak/...
saygo [options] gofiles
```

* *-q* option to disable the speaking if you are just debugging the language processing.
* *-func funcname* to only read out a specific function
* *-o anAudioFile.aiff* to save the speech to a file

Otherwise, just specify Go files on the command-line and it will read out each one.
