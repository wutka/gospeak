package gospeak

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type GoSpeaker interface {
	SpeakGoFile(filename string)
	SpeakGoFunction(filename string, function string)
	SpeakGoString(s string)

	LoadFile(filename string)
	LoadString(s string)

	SpeakAll()
	SpeakFunction(function string)
	SpeakRange(start, end int)

	SetRange(start, end int)
	SetTargetFunction(function string)
}

type goSpeaker struct {
	quiet           bool
	skipImports     bool
	targetFunction  string
	startLine       int
	endLine         int
	audioOutputFile string
	verboseOutput   bool

	speechBuffer strings.Builder
	fileSet      *token.FileSet
	fileBuffer   string

	functionStack []string
	file          *ast.File
}

func MakeGoSpeakerDefault() GoSpeaker {
	return &goSpeaker{
		startLine: -1,
		endLine:   -1,
	}
}

func MakeGoSpeaker(quiet bool, verbose bool, skipImports bool, audioOutputFile string) GoSpeaker {
	return &goSpeaker{
		quiet:           quiet,
		verboseOutput:   verbose,
		skipImports:     skipImports,
		audioOutputFile: audioOutputFile,
		startLine:       -1,
		endLine:         -1,
	}
}

func (gsp *goSpeaker) SpeakGoFile(filename string) {
	gsp.LoadFile(filename)
	if gsp.file != nil {
		gsp.SpeakAll()
	}
}

func (gsp *goSpeaker) SpeakGoFunction(filename string, function string) {
	gsp.LoadFile(filename)
	if gsp.file != nil {
		gsp.SpeakFunction(function)
	}
}

func (gsp *goSpeaker) SpeakGoString(s string) {
	gsp.LoadString(s)
	if gsp.file != nil {
		gsp.SpeakAll()
	}
}

func (gsp *goSpeaker) LoadFile(filename string) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		gsp.speak("I can't find the file named " + speakableFilename(filename))
		fmt.Printf("File %s does not exist\n", filename)
		return
	}

	gsp.fileSet = token.NewFileSet() // positions are relative to fset

	var err error

	gsp.file, err = parser.ParseFile(gsp.fileSet, filename, nil, parser.ParseComments)
	if err != nil && gsp.file == nil {
		panic(err)
	}
	if err != nil {
		fmt.Printf("Warning: file had compile errors: %+v\n", err)
	}
}

func (gsp *goSpeaker) LoadString(s string) {
	gsp.fileBuffer = s

	gsp.fileSet = token.NewFileSet() // positions are relative to fset

	var err error

	gsp.file, err = parser.ParseFile(gsp.fileSet, "buffer", []byte(s), parser.ParseComments)
	if err != nil && gsp.file == nil {
		panic(err)
	}
	if err != nil {
		fmt.Printf("Warning: file had compile errors: %+v\n", err)
	}

}

func (gsp *goSpeaker) SpeakAll() {

	gsp.speakFile(gsp.file)

	gsp.speakBuffer()
}

func (gsp *goSpeaker) SpeakFunction(function string) {
	gsp.targetFunction = function

	gsp.speakFile(gsp.file)

	gsp.speakBuffer()
}

func (gsp *goSpeaker) SpeakRange(start, end int) {
	gsp.startLine = start
	gsp.endLine = end

	gsp.speakFile(gsp.file)

	gsp.speakBuffer()
}

func (gsp *goSpeaker) SetRange(start, end int) {
	gsp.startLine = start
	gsp.endLine = end
}

func (gsp *goSpeaker) SetTargetFunction(function string) {
	gsp.targetFunction = function
}

func (gsp *goSpeaker) GetSpeechString() string {
	return gsp.speechBuffer.String()
}

func (gsp *goSpeaker) isRanged() bool {
	return gsp.targetFunction != "" || (gsp.startLine >= 0 && gsp.endLine >= 0)
}

func (gsp *goSpeaker) isInRange(n ast.Node) bool {
	if gsp.startLine < 0 || gsp.endLine < 0 {
		return true
	}

	if gsp.targetFunction != "" {
		found := false
		for i := len(gsp.functionStack); i >= 0; i-- {
			if gsp.functionStack[i] == gsp.targetFunction {
				found = true
			}
		}

		if !found {
			return false
		}
	}

	startPos := gsp.fileSet.Position(n.Pos())
	endPos := gsp.fileSet.Position(n.End())

	return (startPos.Line >= gsp.startLine && startPos.Line <= gsp.endLine) ||
		(endPos.Line >= gsp.startLine && endPos.Line <= gsp.endLine)
}

func (gsp *goSpeaker) isPosInRange(p token.Pos) bool {
	if gsp.startLine < 0 || gsp.endLine < 0 {
		return true
	}

	pos := gsp.fileSet.Position(p)

	return pos.Line >= gsp.startLine && pos.Line <= gsp.endLine
}

func (gsp *goSpeaker) isStartInRange(n ast.Node) bool {
	startPos := gsp.fileSet.Position(n.Pos())

	return startPos.Line >= gsp.startLine && startPos.Line <= gsp.endLine
}

func (gsp *goSpeaker) isEndInRange(n ast.Node) bool {
	endPos := gsp.fileSet.Position(n.End())

	return endPos.Line >= gsp.startLine && endPos.Line <= gsp.endLine
}

func (gsp *goSpeaker) speakFile(file *ast.File) {

	if file.Name.String() != "" && gsp.isStartInRange(file) {
		gsp.speak("package " + file.Name.String())
	}

	if !gsp.skipImports {
		gsp.speakImportSpecs(file.Imports)
	}

	if !gsp.isRanged() && gsp.startLine < 0 && len(file.Decls) > 0 {
		gsp.speak("declarations")
	}

	for _, d := range file.Decls {
		gsp.speakDeclaration(d)
	}
}

func speakableFilename(filename string) string {
	if strings.HasSuffix(filename, ".go") {
		filename = filename[:len(filename)-3] + " dot go"
	}
	return filename
}

func (gsp *goSpeaker) getFileString(from, to token.Pos) string {
	fromPosition := gsp.fileSet.Position(from)
	toPosition := gsp.fileSet.Position(to)

	bytesToRead := toPosition.Offset - fromPosition.Offset + 1
	if bytesToRead < 0 {
		fmt.Printf("From: %d  To: %d  Negative number of bytes\n",
			fromPosition.Offset, toPosition.Offset)
		return ""
	} else if bytesToRead == 0 {
		return ""
	}

	if gsp.fileBuffer != "" {
		return gsp.fileBuffer[fromPosition.Offset : toPosition.Offset+1]
	}

	f, err := os.Open(fromPosition.Filename)
	if err != nil {
		fmt.Printf("Unable to open %s: %+v", fromPosition.Filename, err)
		return ""
	}
	defer f.Close()

	_, err = f.Seek(int64(fromPosition.Offset), 0)
	if err != nil {
		fmt.Printf("Error seeking in %s: %+v", fromPosition.Filename, err)
		return ""
	}

	buff := make([]byte, bytesToRead)
	n, err := f.Read(buff)
	if err != nil {
		fmt.Printf("Error reading from %s: %+v", fromPosition.Filename, err)
		return ""
	}

	return string(buff[:n])
}

var symbolTranslations = map[string]string{
	"os":      "oh ess",
	"github":  "git hub",
	"fmt":     "fumt",
	"printf":  "print f",
	"sprintf": "s print f",
	"fprintf": "f print f",
	".":       "dot",
	",":       "comma",
	"/":       "slash",
	"\\":      "backslash",
	"utf":     "you tee f",
	"ast":     "eigh s t",
	"a":       "eigh",
	"strconv": "stir conv",
	"_":       "none",
}

func symbolToSpeech(sym string) string {
	splits := splitSymbol(sym)
	trans := translateSymbols(splits)
	return strings.Join(trans, " ")
}

func splitSymbol(symbol string) []string {
	symbols := []string{}
	currSymbol := []byte{}
	runeBuff := make([]byte, 4)
	for _, ch := range symbol {
		if unicode.IsLetter(ch) {
			n := utf8.EncodeRune(runeBuff, ch)
			currSymbol = append(currSymbol, runeBuff[:n]...)
		} else if len(currSymbol) > 0 {
			symbols = append(symbols, string(currSymbol))
			currSymbol = []byte{}
			n := utf8.EncodeRune(runeBuff, ch)
			symbols = append(symbols, string(runeBuff[:n]))
		} else {
			n := utf8.EncodeRune(runeBuff, ch)
			symbols = append(symbols, string(runeBuff[:n]))
		}

	}
	if len(currSymbol) > 0 {
		symbols = append(symbols, string(currSymbol))
	}

	return symbols
}

func (gsp *goSpeaker) speakSymbol(symbol string) {
	gsp.speak(symbolToSpeech(symbol))
}

func (gsp *goSpeaker) speakString(s string) {
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		s = s[1 : len(s)-1]
		s = strings.Replace(s, "\\", " backslash ", -1)
		if len(s) == 0 {
			gsp.speak("empty string")
		} else if len(strings.TrimSpace(s)) == 0 {
			if len(s) == 1 {
				gsp.speak("string with one blank")
			} else {
				gsp.speak(fmt.Sprintf("string of %d blanks", len(s)))
			}
		} else {
			gsp.speak(s)
		}
	} else {
		gsp.speak(s)
	}
}

func translateSymbols(symbols []string) []string {
	newSyms := []string{}
	for _, sym := range symbols {
		newSym, ok := symbolTranslations[strings.ToLower(sym)]
		if ok {
			sym = newSym
		}
		newSyms = append(newSyms, sym)
	}
	return newSyms
}

func (gsp *goSpeaker) speak(speech string) {
	if gsp.verboseOutput {
		fmt.Printf("Saying: %s\n", speech)
	}
	gsp.speechBuffer.WriteString(speech)
	gsp.speechBuffer.WriteString("{pause}\n")
}

func (gsp *goSpeaker) speakBuffer() {
	if gsp.quiet {
		return
	}
	tempFile, err := ioutil.TempFile(".", "gospeech")
	if err != nil {
		fmt.Printf("Unable to create temp file: %+v\n", err)
		return
	}
	tempFile.WriteString(strings.Replace(gsp.speechBuffer.String(), "{pause}", "[[slnc 200]]", -1))
	tempFile.Close()
	defer os.Remove(tempFile.Name())
	var cmd *exec.Cmd
	if gsp.audioOutputFile == "" {
		cmd = exec.Command("/usr/bin/say", "-f", tempFile.Name())
	} else {
		cmd = exec.Command("/usr/bin/say", "-f", tempFile.Name(), "-o", gsp.audioOutputFile)
	}

	err = cmd.Run()
	if err != nil {
		fmt.Printf("Unable to run say: %+v\n", err)
		return
	}
}

func (gsp *goSpeaker) speakImportSpecs(imports []*ast.ImportSpec) {
	if len(imports) == 0 {
		return
	}
	spokeImports := false

	for _, imp := range imports {
		if !gsp.isInRange(imp) {
			continue
		}
		symSpeech := symbolToSpeech(imp.Path.Value)
		if imp.Name != nil {
			symSpeech = symSpeech + " as " + symbolToSpeech(imp.Name.String())
		}
		if !spokeImports {
			gsp.speak("imports")
			spokeImports = true
		}
		gsp.speak(symSpeech)
	}
}

func (gsp *goSpeaker) speakValueSpec(vs *ast.ValueSpec, specType string) {
	if vs.Names != nil && len(vs.Names) > 1 {
		specType = specType + "s"
	}
	if gsp.isStartInRange(vs) {
		gsp.speak(specType)
	}
	for i := range vs.Names {
		if gsp.isInRange(vs.Names[i]) {
			gsp.speakSymbol(vs.Names[i].String())
			gsp.speak("of type ")
		}
		gsp.speakExpr(vs.Type, true)
		if vs.Values != nil && vs.Values[i] != nil {
			if gsp.isInRange(vs.Values[i]) {
				gsp.speak("equals")
			}
			gsp.speakExpr(vs.Values[i], false)
		}
	}
}

func (gsp *goSpeaker) speakTypeSpec(ts *ast.TypeSpec) {
	if gsp.isInRange(ts) {
		gsp.speak("type")
		gsp.speakSymbol(ts.Name.String())
		gsp.speak("is")
	}
	gsp.speakExpr(ts.Type, true)
}

func (gsp *goSpeaker) speakDeclaration(d ast.Decl) {
	switch v := d.(type) {
	case *ast.FuncDecl:
		gsp.functionStack = append(gsp.functionStack, v.Name.String())

		if gsp.isStartInRange(v) {
			gsp.speak("function " + symbolToSpeech(v.Name.String()))
			if gsp.verboseOutput {
				fmt.Printf("function name: %s\n", v.Name.String())
			}
			if v.Recv != nil && v.Recv.List != nil && len(v.Recv.List) > 0 {
				gsp.speakFieldList(v.Recv, "with", "receiver", nil)
			}

			gsp.speakFieldList(v.Type.Params, "taking ", "parameter", v.Type)
			gsp.speakFieldList(v.Type.Results, "and returning ", "value", v.Type)
		}
		gsp.speakBlockStmt(v.Body, "function body", "end function "+symbolToSpeech(v.Name.String()))

		gsp.functionStack = gsp.functionStack[:len(gsp.functionStack)-1]
	case *ast.GenDecl:
		switch v.Tok {
		case token.CONST:
			for _, c := range v.Specs {
				gsp.speakValueSpec(c.(*ast.ValueSpec), "constant")
			}
		case token.VAR:
			for _, v := range v.Specs {
				gsp.speakValueSpec(v.(*ast.ValueSpec), "var")
			}
		case token.TYPE:
			for _, t := range v.Specs {
				gsp.speakTypeSpec(t.(*ast.TypeSpec))
			}
		}
	case *ast.BadDecl:
		if !gsp.isInRange(v) {
			return
		}
		badDeclText := gsp.getFileString(v.From, v.To)
		gsp.speak("Bad declaration")
		gsp.speakSymbol(badDeclText)
	}
}

func (gsp *goSpeaker) speakFieldList(fields *ast.FieldList, takeOrRec string, fieldType string, parent ast.Node) {
	if fields == nil {
		if parent != nil && gsp.isStartInRange(parent) {
			gsp.speak(takeOrRec + " no " + fieldType + "s")
		}
		return
	}
	if fields.NumFields() == 0 {
		if gsp.isStartInRange(fields) {
			gsp.speak(takeOrRec + " no " + fieldType + "s")
		}
	} else if fields.NumFields() == 1 {
		if gsp.isStartInRange(fields) {
			gsp.speak(takeOrRec + strconv.Itoa(fields.NumFields()) + " " + fieldType)
		}
	} else {
		if gsp.isStartInRange(fields) {
			gsp.speak(takeOrRec + strconv.Itoa(fields.NumFields()) + " " + fieldType + "s")
		}
	}
	if fields.List != nil {
		for _, field := range fields.List {
			gsp.speakField(field)
		}
	}
}

func (gsp *goSpeaker) speakField(field *ast.Field) {
	as := "as "
	if len(field.Names) > 1 {
		as = "all as"
	}
	for _, fn := range field.Names {
		if gsp.isInRange(fn) {
			gsp.speak(symbolToSpeech(fn.String()))
		}
	}
	if gsp.isInRange(field.Type) {
		gsp.speak(as)
		gsp.speakExpr(field.Type, true)
	}
	if field.Tag != nil {
		if gsp.isInRange(field.Tag) {
			gsp.speak("with tag")
		}
		gsp.speakExpr(field.Tag, true)
	}
}

func (gsp *goSpeaker) speakExpr(expr ast.Expr, isDecl bool) {
	if expr == nil {
		return
	}
	switch v := expr.(type) {
	case *ast.Ident:
		if gsp.isInRange(v) {
			gsp.speak(symbolToSpeech(v.String()))
		}
	case *ast.ArrayType:
		if gsp.isInRange(v) {
			if v.Len == nil {
				gsp.speak("slice of")
			} else {
				gsp.speakExpr(v.Len, isDecl)
				if gsp.isEndInRange(v.Len) {
					gsp.speak("element")
					gsp.speak("array of")
				}
			}
		}
		gsp.speakExpr(v.Elt, isDecl)
	case *ast.StarExpr:
		if gsp.isInRange(v) {
			if isDecl {
				gsp.speak("pointer to")
			} else {
				gsp.speak("contents of ")
			}
		}
		gsp.speakExpr(v.X, isDecl)
	case *ast.MapType:
		if gsp.isStartInRange(v) {
			gsp.speak("map")
		}
		if gsp.isStartInRange(v.Key) {
			gsp.speak("with ")
		}
		gsp.speakExpr(v.Key, isDecl)
		if gsp.isEndInRange(v.Key) {
			gsp.speak("key")
		}
		if gsp.isStartInRange(v.Value) {
			gsp.speak("and ")
		}
		gsp.speakExpr(v.Value, isDecl)
		if gsp.isEndInRange(v.Value) {
			gsp.speak("value")
		}
	case *ast.SelectorExpr:
		gsp.speakExpr(v.X, isDecl)
		if gsp.isInRange(v.Sel) {
			gsp.speak("dot")
		}
		gsp.speakExpr(v.Sel, isDecl)
	case *ast.BinaryExpr:
		gsp.speakExpr(v.X, isDecl)
		if gsp.isPosInRange(v.OpPos) {
			gsp.speakBinaryOp(v.Op.String())
		}
		gsp.speakExpr(v.Y, isDecl)
	case *ast.ParenExpr:
		if gsp.isPosInRange(v.Lparen) {
			gsp.speak("left paren")
		}
		gsp.speakExpr(v.X, isDecl)
		if gsp.isPosInRange(v.Rparen) {
			gsp.speak("right paren")
		}
	case *ast.CallExpr:
		gsp.speakFunctionCall(v)
	case *ast.UnaryExpr:
		if v.Op.IsOperator() && gsp.isPosInRange(v.OpPos) {
			gsp.speakUnaryOp(v.Op.String())
		}
		gsp.speakExpr(v.X, isDecl)
	case *ast.BasicLit:
		if gsp.isStartInRange(v) {
			gsp.speakString(v.Value)
		}
	case *ast.SliceExpr:
		if gsp.isStartInRange(v) {
			gsp.speak("slice")
		}
		gsp.speakExpr(v.X, isDecl)
		if gsp.isPosInRange(v.Lbrack) {
			gsp.speak("from")
		}
		if v.Low != nil {
			gsp.speakExpr(v.Low, isDecl)
		} else {
			if gsp.isPosInRange(v.Lbrack) {
				gsp.speak("start")
			}
		}

		if v.High != nil {
			if gsp.isInRange(v.High) {
				gsp.speak("to")
			}
			gsp.speakExpr(v.High, isDecl)
		} else {
			if !v.Slice3 && gsp.isPosInRange(v.Rbrack) {
				gsp.speak("to end")
			} else if gsp.isPosInRange(v.Rbrack) {
				gsp.speak("to end")
			}
		}
		if v.Slice3 {
			if gsp.isInRange(v.Max) {
				gsp.speak("with cap ")
			}
			gsp.speakExpr(v.Max, isDecl)
		}
	case *ast.CompositeLit:
		gsp.speakCompositeLit(v, isDecl)

	case *ast.KeyValueExpr:
		if gsp.isInRange(v.Key) {
			gsp.speak("key")
		}
		gsp.speakExpr(v.Key, isDecl)
		if gsp.isInRange(v.Value) {
			gsp.speak("with value	")
		}
		gsp.speakExpr(v.Value, isDecl)

	case *ast.FuncLit:
		if gsp.isStartInRange(v) {
			gsp.speak("lambda")
		}
		gsp.speakFieldList(v.Type.Params, "taking", "parameter", v.Type)
		gsp.speakFieldList(v.Type.Results, "and returning", "value", v.Type)
		gsp.speakBlockStmt(v.Body, "is", "end lambda")

	case *ast.IndexExpr:
		gsp.speakExpr(v.X, isDecl)
		if gsp.isPosInRange(v.Lbrack) {
			gsp.speak("sub")
		}
		gsp.speakExpr(v.Index, isDecl)

	case *ast.InterfaceType:
		gsp.speakInterfaceType(v)

	case *ast.StructType:
		gsp.speakStructType(v)

	case *ast.TypeAssertExpr:
		gsp.speakExpr(v.X, isDecl)
		if v.Type != nil && gsp.isStartInRange(v.Type) || (v.X != nil && gsp.isEndInRange(v.X)) {
			gsp.speak("as type")
		}
		gsp.speakExpr(v.Type, false)

	case *ast.ChanType:
		if v.Dir == ast.SEND {
			if gsp.isPosInRange(v.Arrow) {
				gsp.speak("send to channel")
			}
			gsp.speakExpr(v.Value, isDecl)
		} else {
			if gsp.isPosInRange(v.Arrow) {
				gsp.speak("received from channel")
			}
			gsp.speakExpr(v.Value, isDecl)
		}

	case *ast.Ellipsis:
		if v.Elt != nil {
			if gsp.isStartInRange(v) {
				gsp.speak("variable number of")
			}
			gsp.speakExpr(v.Elt, isDecl)
		} else {
			if gsp.isInRange(v) {
				gsp.speak("variable number")
			}
		}

	case *ast.FuncType:
		if gsp.isStartInRange(v) {
			gsp.speak("function")
		}
		gsp.speakFieldList(v.Params, "taking ", "parameter", v)
		gsp.speakFieldList(v.Results, "and returning ", "value", v)

	case *ast.BadExpr:
		badDeclText := gsp.getFileString(v.From, v.To)
		if gsp.isStartInRange(v) {
			gsp.speak("Bad expression")
		}
		gsp.speakSymbol(badDeclText)
	}
}

func (gsp *goSpeaker) speakCompositeLit(c *ast.CompositeLit, isDecl bool) {
	if len(c.Elts) == 0 {
		if gsp.isStartInRange(c) {
			gsp.speak("empty")
		}
	}
	if c.Type != nil {
		gsp.speakExpr(c.Type, isDecl)
	}
	if len(c.Elts) > 0 {
		if gsp.isPosInRange(c.Lbrace) {
			gsp.speak("containing")
		}
	}
	first := true
	for _, e := range c.Elts {
		if !first {
			if gsp.isStartInRange(e) {
				gsp.speak("comma")
			}
		} else {
			first = false
		}
		gsp.speakExpr(e, isDecl)
	}
}

func (gsp *goSpeaker) speakInterfaceType(iface *ast.InterfaceType) {
	if iface.Methods == nil || iface.Methods.List == nil || len(iface.Methods.List) == 0 {
		if gsp.isInRange(iface) {
			gsp.speak("empty interface")
		}
	} else {
		if gsp.isInRange(iface) {
			gsp.speak("interface")
		}
		gsp.speakFieldList(iface.Methods, "having", "method", iface)
	}
}

func (gsp *goSpeaker) speakStructType(s *ast.StructType) {
	if s.Fields == nil || s.Fields.List == nil || len(s.Fields.List) == 0 {
		if gsp.isStartInRange(s) {
			gsp.speak("empty struct")
		}
	} else {
		if gsp.isStartInRange(s) {
			gsp.speak("struct")
		}
		gsp.speakFieldList(s.Fields, "having", "field", s)
	}
}

func (gsp *goSpeaker) speakFunctionCall(c *ast.CallExpr) {
	if len(c.Args) == 0 {
		if gsp.isStartInRange(c) {
			gsp.speak("call")
		}
	}
	gsp.speakExpr(c.Fun, false)
	if len(c.Args) > 0 {
		if gsp.isPosInRange(c.Lparen) {
			gsp.speak("of")
		}
	}
	spokeEllipsis := false
	first := true
	for _, a := range c.Args {
		if !first {
			if gsp.isStartInRange(a) {
				gsp.speak("comma	")
			}
		} else {
			first = false
		}
		if c.Ellipsis != token.NoPos && c.Ellipsis < a.Pos() && !spokeEllipsis {
			if gsp.isPosInRange(c.Ellipsis) {
				gsp.speak("ellipsis")
			}
			spokeEllipsis = true
		}
		gsp.speakExpr(a, false)
	}
}

var binaryOpSpeech = map[string]string{
	"||": "or",
	"&&": "and",
	"==": "equals",
	"!=": "does not equal",
	"<":  "is less than",
	"<=": "is less than or equal to",
	">":  "is greater than",
	">=": "is greater than or equal to",
	"+":  "plus",
	"-":  "minus",
	"|":  "bitwise or",
	"^":  "exclusive or",
	"*":  "times",
	"/":  "divided by",
	"%":  "modulo",
	"<<": "shifted left by",
	">>": "shifted right by",
	"&":  "bitwise and",
	"&^": "bitwise and not",
}

func (gsp *goSpeaker) speakBinaryOp(op string) {
	speechVal, ok := binaryOpSpeech[op]
	if ok {
		gsp.speak(speechVal)
	}
}

var unaryOpSpeech = map[string]string{
	"+":  "positive",
	"-":  "negative",
	"!":  "not",
	"^":  "bitwise not",
	"*":  "star",
	"&":  "ref",
	"<-": "receive from channel",
}

func (gsp *goSpeaker) speakUnaryOp(op string) {
	speechVal, ok := unaryOpSpeech[op]
	if ok {
		gsp.speak(speechVal)
	}
}

func (gsp *goSpeaker) speakBlockStmt(stmts *ast.BlockStmt, bodyStart string, bodyEnd string) {
	if gsp.isStartInRange(stmts) {
		gsp.speak(bodyStart)
	}
	for _, bs := range stmts.List {
		gsp.speakStmt(bs)
	}
	if gsp.isEndInRange(stmts) {
		gsp.speak(bodyEnd)
	}
}

func (gsp *goSpeaker) speakStmt(stmt ast.Stmt) {
	switch v := stmt.(type) {
	case *ast.BlockStmt:
		if gsp.isInRange(stmt) {
			gsp.speak("begin block")
		}
		for _, bs := range v.List {
			gsp.speakStmt(bs)
		}
		if gsp.isInRange(stmt) {
			gsp.speak("end block")
		}
	case *ast.IfStmt:
		gsp.speakIfStatement(v)
	case *ast.ForStmt:
		gsp.speakForLoop(v)
	case *ast.RangeStmt:
		if gsp.isStartInRange(v) {
			gsp.speak("range over ")
		}
		gsp.speakExpr(v.X, false)
		if (v.Key != nil && gsp.isStartInRange(v.Key)) || (v.Key == nil && v.Value != nil &&
			gsp.isStartInRange(v.Value)) {
			gsp.speak("with")
		}
		if v.Key != nil {
			if gsp.isStartInRange(v.Key) {
				gsp.speak("key")
			}
			gsp.speakExpr(v.Key, false)
			if v.Value != nil {
				if gsp.isStartInRange(v.Value) {
					gsp.speak("and")
				}
			}
		}
		if v.Value != nil {
			if gsp.isInRange(v.Value) {
				gsp.speak("value")
			}
			gsp.speakExpr(v.Value, false)
		}
		if v.Body != nil {
			gsp.speakBlockStmt(v.Body, "range body", "end range")
		}
	case *ast.ReturnStmt:
		if gsp.isStartInRange(v) {
			gsp.speak("return")
		}

		first := true
		for _, e := range v.Results {
			if !first {
				if gsp.isStartInRange(e) {
					gsp.speak("also")
				}
			} else {
				first = false
			}
			gsp.speakExpr(e, false)
		}
	case *ast.AssignStmt:
		gsp.speakAssignStatement(v)

	case *ast.ExprStmt:
		gsp.speakExpr(v.X, false)

	case *ast.BranchStmt:
		if gsp.isStartInRange(v) {
			gsp.speak(v.Tok.String())
		}
		if v.Label != nil {
			if gsp.isInRange(v.Label) {
				gsp.speak("at")
			}
			gsp.speakSymbol(v.Label.String())
		}
	case *ast.SwitchStmt:
		gsp.speakSwitchStatement(v)

	case *ast.TypeSwitchStmt:
		gsp.speakTypeSwitchStatement(v)

	case *ast.CommClause:
		gsp.speakCommClause(v)

	case *ast.CaseClause:
		gsp.speakSwitchCase(v)

	case *ast.DeferStmt:
		if gsp.isStartInRange(v) {
			gsp.speak("defer")
		}
		gsp.speakExpr(v.Call, false)

	case *ast.GoStmt:
		if gsp.isStartInRange(v) {
			gsp.speak("go")
		}
		gsp.speakExpr(v.Call, false)

	case *ast.EmptyStmt:
		if gsp.isInRange(v) {
			gsp.speak("empty")
		}

	case *ast.IncDecStmt:
		if gsp.isStartInRange(v) {
			if v.Tok == token.INC {
				gsp.speak("increment")
			} else {
				gsp.speak("decrement")
			}
		}
		gsp.speakExpr(v.X, false)

	case *ast.LabeledStmt:
		if gsp.isStartInRange(v) {
			gsp.speak("label")
			gsp.speakSymbol(v.Label.String())
		}
		gsp.speakStmt(v.Stmt)

	case *ast.SelectStmt:
		gsp.speakSelectStatement(v)

	case *ast.SendStmt:
		if gsp.isStartInRange(v) {
			gsp.speak("send")
		}
		gsp.speakExpr(v.Value, false)
		if gsp.isInRange(v.Chan) {
			gsp.speak("to channel")
		}
		gsp.speakExpr(v.Chan, false)

	case *ast.BadStmt:
		badDeclText := gsp.getFileString(v.From, v.To)
		if gsp.isStartInRange(v) {
			gsp.speak("Bad statement")
		}
		gsp.speakSymbol(badDeclText)

	case *ast.DeclStmt:
		gsp.speakDeclaration(v.Decl)
	}
}

func (gsp *goSpeaker) speakAssignStatement(s *ast.AssignStmt) {
	if gsp.isStartInRange(s) {
		gsp.speak("let")
	}
	if len(s.Lhs) > 1 && len(s.Lhs) == len(s.Rhs) {
		for i := range s.Lhs {
			gsp.speakExpr(s.Lhs[i], false)
			if gsp.isEndInRange(s.Lhs[i]) {
				gsp.speak("equal")
			}
			gsp.speakExpr(s.Rhs[i], false)
		}
	} else {
		first := true
		for _, l := range s.Lhs {
			if !first {
				if gsp.isStartInRange(l) {
					gsp.speak("and")
				}
			} else {
				first = false
			}
			gsp.speakExpr(l, false)
		}
		if len(s.Rhs) > 0 && gsp.isStartInRange(s.Rhs[0]) {
			gsp.speak("equal")
		}
		for _, r := range s.Rhs {
			gsp.speakExpr(r, false)
		}
	}
}

func (gsp *goSpeaker) speakIfStatement(s *ast.IfStmt) {
	if gsp.isStartInRange(s) {
		gsp.speak("if")
	}
	if s.Init != nil {
		if gsp.isStartInRange(s.Init) {
			gsp.speak("with initializer ")
		}
		gsp.speakStmt(s.Init)
		if gsp.isEndInRange(s.Init) {
			gsp.speak("when")
		}
	}
	if s.Cond != nil {
		gsp.speakExpr(s.Cond, false)
	}
	if s.Body != nil {
		bodyEnd := "end if"
		if s.Else != nil {
			bodyEnd = ""
		}
		gsp.speakBlockStmt(s.Body, "then", bodyEnd)
	}
	if s.Else != nil {
		switch e := s.Else.(type) {
		case *ast.BlockStmt:
			gsp.speakBlockStmt(e, "else", "end if")
		default:
			if e != nil && gsp.isStartInRange(e) {
				gsp.speak("else")
			}
			gsp.speakStmt(e)
		}
	}
}
func (gsp *goSpeaker) speakForLoop(fl *ast.ForStmt) {
	loopType := "for"
	if fl.Init == nil && fl.Post == nil {
		if fl.Cond == nil {
			if gsp.isStartInRange(fl) {
				gsp.speak("for ever")
			}
		} else {
			if gsp.isStartInRange(fl) {
				gsp.speak("while")
			}
			loopType = "while"
			gsp.speakExpr(fl.Cond, false)
		}
	} else {
		if gsp.isStartInRange(fl) {
			gsp.speak("for")
		}
		if fl.Init == nil {
			gsp.speakStmt(fl.Init)
		}
		if fl.Cond != nil {
			if gsp.isStartInRange(fl.Cond) {
				gsp.speak("while")
			}
			gsp.speakExpr(fl.Cond, false)
		}
		if fl.Post != nil {
			gsp.speakStmt(fl.Post)
		}
	}
	gsp.speakBlockStmt(fl.Body, "do", "end "+loopType+" loop")
}

func (gsp *goSpeaker) speakSwitchStatement(s *ast.SwitchStmt) {
	if gsp.isStartInRange(s) {
		gsp.speak("switch")
	}
	if s.Init != nil {
		if gsp.isStartInRange(s.Init) {
			gsp.speak("with initializer")
		}
		gsp.speakStmt(s.Init)
	}
	if s.Tag != nil && gsp.isStartInRange(s.Tag) {
		gsp.speak("on")
	}
	gsp.speakExpr(s.Tag, false)
	gsp.speakBlockStmt(s.Body, "", "end switch")

}

func (gsp *goSpeaker) speakTypeSwitchStatement(s *ast.TypeSwitchStmt) {
	if gsp.isStartInRange(s) {
		gsp.speak("switch")
	}
	if s.Init != nil {
		if gsp.isStartInRange(s.Init) {
			gsp.speak("with initializer")
		}
		gsp.speakStmt(s.Init)
	}

	if gsp.isStartInRange(s.Assign) {
		gsp.speak("on type")
	}
	gsp.speakStmt(s.Assign)
	gsp.speakBlockStmt(s.Body, "", "end type switch")

}

func (gsp *goSpeaker) speakCommClause(c *ast.CommClause) {
	if gsp.isStartInRange(c) {
		if c.Comm != nil {
			gsp.speak("default")
		} else {
			gsp.speak("case")
		}
	}
	gsp.speakStmt(c.Comm)
	for _, cs := range c.Body {
		gsp.speakStmt(cs)
	}
}

func (gsp *goSpeaker) speakSwitchCase(c *ast.CaseClause) {
	if gsp.isStartInRange(c) {
		if len(c.List) == 0 {
			gsp.speak("default")
		} else {
			gsp.speak("case")
		}
	}
	first := true
	for _, e := range c.List {
		if !first {
			if gsp.isStartInRange(e) {
				gsp.speak("or")
			}
		} else {
			first = false
		}
		gsp.speakExpr(e, false)
	}
	for _, cs := range c.Body {
		gsp.speakStmt(cs)
	}
}

func (gsp *goSpeaker) speakSelectStatement(s *ast.SelectStmt) {
	if gsp.isStartInRange(s) {
		gsp.speak("select")
	}
	gsp.speakBlockStmt(s.Body, "", "end select")

}
