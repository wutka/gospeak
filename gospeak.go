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
}

type goSpeaker struct {
	quiet           bool
	skipImports     bool
	targetFunction  string
	audioOutputFile string
	verboseOutput   bool

	speechBuffer strings.Builder
	fileSet      *token.FileSet
}

func MakeGoSpeakerDefault() GoSpeaker {
	return &goSpeaker{}
}

func MakeGoSpeaker(quiet bool, verbose bool, skipImports bool, audioOutputFile string) GoSpeaker {
	return &goSpeaker{
		quiet:           quiet,
		verboseOutput:   verbose,
		skipImports:     skipImports,
		audioOutputFile: audioOutputFile,
	}
}

func (gsp *goSpeaker) SpeakGoFile(filename string) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		gsp.speak("I can't find the file named " + speakableFilename(filename))
		fmt.Printf("File %s does not exist\n", filename)
		return
	}

	gsp.fileSet = token.NewFileSet() // positions are relative to fset

	f, err := parser.ParseFile(gsp.fileSet, filename, nil, parser.ParseComments)
	if err != nil && f == nil {
		panic(err)
	}
	if err != nil {
		fmt.Printf("Warning: file had compile errors: %+v", err)
	}

	gsp.speakFile(f)

	gsp.speakBuffer()
}

func (gsp *goSpeaker) SpeakGoFunction(filename string, function string) {
	gsp.targetFunction = function

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		gsp.speak("I can't find the file named " + speakableFilename(filename))
		fmt.Printf("File %s does not exist\n", filename)
		return
	}

	gsp.fileSet = token.NewFileSet() // positions are relative to fset

	f, err := parser.ParseFile(gsp.fileSet, filename, nil, parser.ParseComments)
	if err != nil && f == nil {
		panic(err)
	}
	if err != nil {
		fmt.Printf("Warning: file had compile errors: %+v", err)
	}

	gsp.speakFile(f)

	gsp.speakBuffer()
}

func (gsp *goSpeaker) speakFile(file *ast.File) {

	if gsp.targetFunction == "" {
		gsp.speak("package " + file.Name.String())
	}

	if gsp.targetFunction == "" && !gsp.skipImports {
		gsp.speakImportSpecs(file.Imports)
	}

	if gsp.targetFunction == "" {
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
		newSym, ok := symbolTranslations[sym]
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
	if gsp.quiet {
		return
	}
	gsp.speechBuffer.WriteString(speech)
	gsp.speechBuffer.WriteString("[[slnc 200]]\n")
}

func (gsp *goSpeaker) speakBuffer() {
	tempFile, err := ioutil.TempFile(".", "gospeech")
	if err != nil {
		fmt.Printf("Unable to create temp file: %+v\n", err)
		return
	}
	tempFile.WriteString(gsp.speechBuffer.String())
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
	gsp.speak("imports")

	for _, imp := range imports {
		symSpeech := symbolToSpeech(imp.Path.Value)
		if imp.Name != nil {
			symSpeech = symSpeech + " as " + symbolToSpeech(imp.Name.String())
		}
		gsp.speak(symSpeech)
	}
}

func (gsp *goSpeaker) speakValueSpec(vs *ast.ValueSpec, specType string) {
	if vs.Names != nil && len(vs.Names) > 1 {
		specType = specType + "s"
	}
	gsp.speak(specType)
	for i := range vs.Names {
		gsp.speakSymbol(vs.Names[i].String())
		gsp.speak("of type ")
		gsp.speakExpr(vs.Type)
		if vs.Values != nil && vs.Values[i] != nil {
			gsp.speak("equals")
			gsp.speakExpr(vs.Values[i])
		}
	}
}

func (gsp *goSpeaker) speakTypeSpec(ts *ast.TypeSpec) {
	gsp.speak("type")
	gsp.speakSymbol(ts.Name.String())
	gsp.speak("is")
	gsp.speakExpr(ts.Type)
}

func (gsp *goSpeaker) speakDeclaration(d ast.Decl) {
	switch v := d.(type) {
	case *ast.FuncDecl:
		if gsp.targetFunction != "" && gsp.targetFunction != v.Name.String() {
			return
		}
		gsp.speak("function " + symbolToSpeech(v.Name.String()))
		if gsp.verboseOutput {
			fmt.Printf("function name: %s\n", v.Name.String())
		}
		gsp.speakFieldList(v.Type.Params, "taking ", "parameter")
		gsp.speakFieldList(v.Type.Results, "and returning ", "value")
		gsp.speakBlockStmt(v.Body, "function body", "end function "+symbolToSpeech(v.Name.String()))
	case *ast.GenDecl:
		if gsp.targetFunction != "" {
			return
		}
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
		badDeclText := gsp.getFileString(v.From, v.To)
		gsp.speak("Bad declaration")
		gsp.speakSymbol(badDeclText)
	}
}

func (gsp *goSpeaker) speakFieldList(fields *ast.FieldList, takeOrRec string, fieldType string) {
	if fields == nil {
		gsp.speak(takeOrRec + " no " + fieldType + "s")
		return
	}
	if fields.NumFields() == 0 {
		gsp.speak(takeOrRec + " no " + fieldType + "s")
	} else if fields.NumFields() == 1 {
		gsp.speak(takeOrRec + strconv.Itoa(fields.NumFields()) + " " + fieldType)
	} else {
		gsp.speak(takeOrRec + strconv.Itoa(fields.NumFields()) + " " + fieldType + "s")
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
		gsp.speak(symbolToSpeech(fn.String()))
	}
	gsp.speak(as)
	gsp.speakExpr(field.Type)
	if field.Tag != nil {
		gsp.speak("with tag")
		gsp.speakExpr(field.Tag)
	}
}

func (gsp *goSpeaker) speakExpr(expr ast.Expr) {
	if expr == nil {
		return
	}
	switch v := expr.(type) {
	case *ast.Ident:
		gsp.speak(symbolToSpeech(v.String()))
	case *ast.ArrayType:
		if v.Len == nil {
			gsp.speak("slice of")
		} else {
			gsp.speakExpr(v.Len)
			gsp.speak("element")
			gsp.speak("array of")
		}
		gsp.speakExpr(v.Elt)
	case *ast.StarExpr:
		gsp.speak("pointer to")
		gsp.speakExpr(v.X)
	case *ast.MapType:
		gsp.speak("map with ")
		gsp.speakExpr(v.Key)
		gsp.speak("key and ")
		gsp.speakExpr(v.Value)
		gsp.speak("value")
	case *ast.SelectorExpr:
		gsp.speakExpr(v.X)
		gsp.speak("dot")
		gsp.speakExpr(v.Sel)
	case *ast.BinaryExpr:
		gsp.speakExpr(v.X)
		gsp.speakBinaryOp(v.Op.String())
		gsp.speakExpr(v.Y)
	case *ast.ParenExpr:
		gsp.speak("left paren")
		gsp.speakExpr(v.X)
		gsp.speak("right paren")
	case *ast.CallExpr:
		gsp.speakFunctionCall(v)
	case *ast.UnaryExpr:
		if v.Op.IsOperator() {
			gsp.speakUnaryOp(v.Op.String())
		}
		gsp.speakExpr(v.X)
	case *ast.BasicLit:
		gsp.speakString(v.Value)
	case *ast.SliceExpr:
		gsp.speak("slice")
		gsp.speakExpr(v.X)
		gsp.speak("from")
		if v.Low != nil {
			gsp.speakExpr(v.Low)
		} else {
			gsp.speak("start")
		}
		gsp.speak("to")
		if v.High != nil {
			gsp.speakExpr(v.High)
		} else {
			gsp.speak("end")
		}
		if v.Slice3 {
			gsp.speak("with cap ")
			gsp.speakExpr(v.Max)
		}
	case *ast.CompositeLit:
		gsp.speakCompositeLit(v)

	case *ast.KeyValueExpr:
		gsp.speakExpr(v.Key)
		gsp.speak("colon	")
		gsp.speakExpr(v.Value)

	case *ast.FuncLit:
		gsp.speak("lambda")
		gsp.speakFieldList(v.Type.Params, "taking", "parameter")
		gsp.speakFieldList(v.Type.Results, "and returning", "value")
		gsp.speakBlockStmt(v.Body, "is", "end lambda")

	case *ast.IndexExpr:
		gsp.speakExpr(v.X)
		gsp.speak("sub")
		gsp.speakExpr(v.Index)

	case *ast.InterfaceType:
		gsp.speakInterfaceType(v)

	case *ast.StructType:
		gsp.speakStructType(v)

	case *ast.TypeAssertExpr:
		gsp.speakExpr(v.X)
		gsp.speak("as type")
		gsp.speakExpr(v.Type)

	case *ast.ChanType:
		if v.Dir == ast.SEND {
			gsp.speak("send to channel")
			gsp.speakExpr(v.Value)
		} else {
			gsp.speak("received from channel")
			gsp.speakExpr(v.Value)
		}

	case *ast.Ellipsis:
		if v.Elt != nil {
			gsp.speak("variable number of")
			gsp.speakExpr(v.Elt)
		} else {
			gsp.speak("variable number")
		}

	case *ast.FuncType:
		gsp.speak("function")
		gsp.speakFieldList(v.Params, "taking ", "parameter")
		gsp.speakFieldList(v.Results, "and returning ", "value")

	case *ast.BadExpr:
		badDeclText := gsp.getFileString(v.From, v.To)
		gsp.speak("Bad expression")
		gsp.speakSymbol(badDeclText)
	}
}

func (gsp *goSpeaker) speakCompositeLit(c *ast.CompositeLit) {
	if len(c.Elts) == 0 {
		gsp.speak("empty")
	}
	if c.Type != nil {
		gsp.speakExpr(c.Type)
	}
	if len(c.Elts) > 0 {
		gsp.speak("containing")
	}
	first := true
	for _, e := range c.Elts {
		if !first {
			gsp.speak("comma")
		} else {
			first = false
		}
		gsp.speakExpr(e)
	}
}

func (gsp *goSpeaker) speakInterfaceType(iface *ast.InterfaceType) {
	if iface.Methods == nil || iface.Methods.List == nil || len(iface.Methods.List) == 0 {
		gsp.speak("empty interface")
	} else {
		gsp.speak("interface")
		gsp.speakFieldList(iface.Methods, "having", "method")
	}
}

func (gsp *goSpeaker) speakStructType(s *ast.StructType) {
	if s.Fields == nil || s.Fields.List == nil || len(s.Fields.List) == 0 {
		gsp.speak("empty struct")
	} else {
		gsp.speak("struct")
		gsp.speakFieldList(s.Fields, "having", "field")
	}
}

func (gsp *goSpeaker) speakFunctionCall(c *ast.CallExpr) {
	if len(c.Args) == 0 {
		gsp.speak("call")
	}
	gsp.speakExpr(c.Fun)
	if len(c.Args) > 0 {
		gsp.speak("of")
	}
	spokeEllipsis := false
	first := true
	for _, a := range c.Args {
		if !first {
			gsp.speak("comma	")
		} else {
			first = false
		}
		if c.Ellipsis != token.NoPos && c.Ellipsis < a.Pos() && !spokeEllipsis {
			gsp.speak("ellipsis")
			spokeEllipsis = true
		}
		gsp.speakExpr(a)
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
	gsp.speak(bodyStart)
	for _, bs := range stmts.List {
		gsp.speakStmt(bs)
	}
	gsp.speak(bodyEnd)
}

func (gsp *goSpeaker) speakStmt(stmt ast.Stmt) {
	switch v := stmt.(type) {
	case *ast.BlockStmt:
		gsp.speak("begin block")
		for _, bs := range v.List {
			gsp.speakStmt(bs)
		}
		gsp.speak("end block")
	case *ast.IfStmt:
		gsp.speakIfStatement(v)
	case *ast.ForStmt:
		gsp.speakForLoop(v)
	case *ast.RangeStmt:
		gsp.speak("range over ")
		gsp.speakExpr(v.X)
		gsp.speak("with")
		if v.Key != nil {
			gsp.speak("key")
			gsp.speakExpr(v.Key)
			if v.Value != nil {
				gsp.speak("and")
			}
		}
		if v.Value != nil {
			gsp.speak("value")
			gsp.speakExpr(v.Value)
		}
		if v.Body != nil {
			gsp.speakBlockStmt(v.Body, "range body", "end range")
		}
	case *ast.ReturnStmt:
		gsp.speak("return")
		first := true
		for _, e := range v.Results {
			if !first {
				gsp.speak("also")
			} else {
				first = false
			}
			gsp.speakExpr(e)
		}
	case *ast.AssignStmt:
		gsp.speakAssignStatement(v)

	case *ast.ExprStmt:
		gsp.speakExpr(v.X)

	case *ast.BranchStmt:
		gsp.speak(v.Tok.String())
		if v.Label != nil {
			gsp.speak("at")
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
		gsp.speak("defer")
		gsp.speakExpr(v.Call)

	case *ast.GoStmt:
		gsp.speak("go")
		gsp.speakExpr(v.Call)

	case *ast.EmptyStmt:
		gsp.speak("empty")

	case *ast.IncDecStmt:
		if v.Tok == token.INC {
			gsp.speak("increment")
		} else {
			gsp.speak("decrement")
		}
		gsp.speakExpr(v.X)

	case *ast.LabeledStmt:
		gsp.speak("label")
		gsp.speakSymbol(v.Label.String())
		gsp.speakStmt(v.Stmt)

	case *ast.SelectStmt:
		gsp.speakSelectStatement(v)

	case *ast.SendStmt:
		gsp.speak("send")
		gsp.speakExpr(v.Value)
		gsp.speak("to channel")
		gsp.speakExpr(v.Chan)

	case *ast.BadStmt:
		badDeclText := gsp.getFileString(v.From, v.To)
		gsp.speak("Bad statement")
		gsp.speakSymbol(badDeclText)

	case *ast.DeclStmt:
		gsp.speakDeclaration(v.Decl)
	}
}

func (gsp *goSpeaker) speakAssignStatement(s *ast.AssignStmt) {
	gsp.speak("let")
	if len(s.Lhs) > 1 && len(s.Lhs) == len(s.Rhs) {
		for i := range s.Lhs {
			gsp.speakExpr(s.Lhs[i])
			gsp.speak("equal")
			gsp.speakExpr(s.Rhs[i])
		}
	} else {
		first := true
		for _, l := range s.Lhs {
			if !first {
				gsp.speak("and")
			} else {
				first = false
			}
			gsp.speakExpr(l)
		}
		gsp.speak("equal")
		for _, r := range s.Rhs {
			gsp.speakExpr(r)
		}
	}
}

func (gsp *goSpeaker) speakIfStatement(s *ast.IfStmt) {
	gsp.speak("if")
	if s.Init != nil {
		gsp.speak("with initializer ")
		gsp.speakStmt(s.Init)
		gsp.speak("when")
	}
	if s.Cond != nil {
		gsp.speakExpr(s.Cond)
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
			gsp.speakStmt(e)
		}
	}
}
func (gsp *goSpeaker) speakForLoop(fl *ast.ForStmt) {
	loopType := "for"
	if fl.Init == nil && fl.Post == nil {
		if fl.Cond == nil {
			gsp.speak("for ever")
		} else {
			gsp.speak("while")
			loopType = "while"
			gsp.speakExpr(fl.Cond)
		}
	} else {
		gsp.speak("for")
		if fl.Init == nil {
			gsp.speakStmt(fl.Init)
		}
		if fl.Cond != nil {
			gsp.speak("while")
			gsp.speakExpr(fl.Cond)
		}
		if fl.Post != nil {
			gsp.speakStmt(fl.Post)
		}
	}
	gsp.speakBlockStmt(fl.Body, "do", "end "+loopType+" loop")
}

func (gsp *goSpeaker) speakSwitchStatement(s *ast.SwitchStmt) {
	gsp.speak("switch")
	if s.Init != nil {
		gsp.speak("with initializer")
		gsp.speakStmt(s.Init)
	}
	gsp.speak("on")
	gsp.speakExpr(s.Tag)
	gsp.speakBlockStmt(s.Body, "", "end switch")

}

func (gsp *goSpeaker) speakTypeSwitchStatement(s *ast.TypeSwitchStmt) {
	gsp.speak("switch")
	if s.Init != nil {
		gsp.speak("with initializer")
		gsp.speakStmt(s.Init)
	}

	gsp.speak("on type")
	gsp.speakStmt(s.Assign)
	gsp.speakBlockStmt(s.Body, "", "end type switch")

}

func (gsp *goSpeaker) speakCommClause(c *ast.CommClause) {
	if c.Comm != nil {
		gsp.speak("default")
	} else {
		gsp.speak("case")
	}
	gsp.speakStmt(c.Comm)
	for _, cs := range c.Body {
		gsp.speakStmt(cs)
	}
}

func (gsp *goSpeaker) speakSwitchCase(c *ast.CaseClause) {
	if len(c.List) == 0 {
		gsp.speak("default")
	} else {
		gsp.speak("case")
	}
	first := true
	for _, e := range c.List {
		if !first {
			gsp.speak("or")
		} else {
			first = false
		}
		gsp.speakExpr(e)
	}
	for _, cs := range c.Body {
		gsp.speakStmt(cs)
	}
}

func (gsp *goSpeaker) speakSelectStatement(s *ast.SelectStmt) {
	gsp.speak("select")
	gsp.speakBlockStmt(s.Body, "", "end select")

}
