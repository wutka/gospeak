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

var ShutUp bool
var SkipImports bool
var TargetFunction string
var SayOut string
var VerboseOutput bool

var speechBuffer strings.Builder
var fileSet *token.FileSet

func SpeakGoFile(filename string) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		speak("I can't find the file named " + speakableFilename(filename))
		fmt.Printf("File %s does not exist\n", filename)
		return
	}

	fileSet = token.NewFileSet() // positions are relative to fset

	f, err := parser.ParseFile(fileSet, filename, nil, parser.ParseComments)
	if err != nil && f == nil {
		panic(err)
	}
	if err != nil {
		fmt.Printf("Warning: file had compile errors: %+v", err)
	}

	if TargetFunction == "" {
		speak("package " + f.Name.String())
	}

	if TargetFunction == "" && !SkipImports {
		speakImportSpecs(f.Imports)
	}

	if TargetFunction == "" {
		speak("declarations")
	}

	for _, d := range f.Decls {
		speakDeclaration(d)
	}

	speakBuffer()
}

func speakableFilename(filename string) string {
	if strings.HasSuffix(filename, ".go") {
		filename = filename[:len(filename)-3] + " dot go"
	}
	return filename
}

func getFileString(from, to token.Pos) string {
	fromPosition := fileSet.Position(from)
	toPosition := fileSet.Position(to)

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

func speakSymbol(symbol string) {
	speak(symbolToSpeech(symbol))
}

func speakString(s string) {
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		s = s[1 : len(s)-1]
		s = strings.Replace(s, "\\", " backslash ", -1)
		if len(s) == 0 {
			speak("empty string")
		} else if len(strings.TrimSpace(s)) == 0 {
			if len(s) == 1 {
				speak("string with one blank")
			} else {
				speak(fmt.Sprintf("string of %d blanks", len(s)))
			}
		} else {
			speak(s)
		}
	} else {
		speak(s)
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

func speak(speech string) {
	if VerboseOutput {
		fmt.Printf("Saying: %s\n", speech)
	}
	if ShutUp {
		return
	}
	speechBuffer.WriteString(speech)
	speechBuffer.WriteString("[[slnc 200]]\n")
}

func speakBuffer() {
	tempFile, err := ioutil.TempFile(".", "gospeech")
	if err != nil {
		fmt.Printf("Unable to create temp file: %+v\n", err)
		return
	}
	tempFile.WriteString(speechBuffer.String())
	tempFile.Close()
	defer os.Remove(tempFile.Name())
	var cmd *exec.Cmd
	if SayOut == "" {
		cmd = exec.Command("/usr/bin/say", "-f", tempFile.Name())
	} else {
		cmd = exec.Command("/usr/bin/say", "-f", tempFile.Name(), "-o", SayOut)
	}

	err = cmd.Run()
	if err != nil {
		fmt.Printf("Unable to run say: %+v\n", err)
		return
	}
}

func speakImportSpecs(imports []*ast.ImportSpec) {
	speak("imports")

	for _, imp := range imports {
		symSpeech := symbolToSpeech(imp.Path.Value)
		if imp.Name != nil {
			symSpeech = symSpeech + " as " + symbolToSpeech(imp.Name.String())
		}
		speak(symSpeech)
	}
}

func speakValueSpec(vs *ast.ValueSpec, specType string) {
	if vs.Names != nil && len(vs.Names) > 1 {
		specType = specType + "s"
	}
	speak(specType)
	for i := range vs.Names {
		speakSymbol(vs.Names[i].String())
		speak("of type ")
		speakExpr(vs.Type)
		if vs.Values != nil && vs.Values[i] != nil {
			speak("equals")
			speakExpr(vs.Values[i])
		}
	}
}

func speakTypeSpec(ts *ast.TypeSpec) {
	speak("type")
	speakSymbol(ts.Name.String())
	speak("is")
	speakExpr(ts.Type)
}

func speakDeclaration(d ast.Decl) {
	switch v := d.(type) {
	case *ast.FuncDecl:
		if TargetFunction != "" && TargetFunction != v.Name.String() {
			return
		}
		speak("function " + symbolToSpeech(v.Name.String()))
		if VerboseOutput {
			fmt.Printf("function name: %s\n", v.Name.String())
		}
		speakFieldList(v.Type.Params, "taking ", "parameter")
		speakFieldList(v.Type.Results, "and returning ", "value")
		speakBlockStmt(v.Body, "function body", "end function "+symbolToSpeech(v.Name.String()))
	case *ast.GenDecl:
		if TargetFunction != "" {
			return
		}
		switch v.Tok {
		case token.CONST:
			for _, c := range v.Specs {
				speakValueSpec(c.(*ast.ValueSpec), "constant")
			}
		case token.VAR:
			for _, v := range v.Specs {
				speakValueSpec(v.(*ast.ValueSpec), "var")
			}
		case token.TYPE:
			for _, t := range v.Specs {
				speakTypeSpec(t.(*ast.TypeSpec))
			}
		}
	case *ast.BadDecl:
		badDeclText := getFileString(v.From, v.To)
		speak("Bad declaration")
		speakSymbol(badDeclText)
	}
}

func speakFieldList(fields *ast.FieldList, takeOrRec string, fieldType string) {
	if fields == nil {
		speak(takeOrRec + " no " + fieldType + "s")
		return
	}
	if fields.NumFields() == 0 {
		speak(takeOrRec + " no " + fieldType + "s")
	} else if fields.NumFields() == 1 {
		speak(takeOrRec + strconv.Itoa(fields.NumFields()) + " " + fieldType)
	} else {
		speak(takeOrRec + strconv.Itoa(fields.NumFields()) + " " + fieldType + "s")
	}
	if fields.List != nil {
		for _, field := range fields.List {
			speakField(field)
		}
	}
}

func speakField(field *ast.Field) {
	as := "as "
	if len(field.Names) > 1 {
		as = "all as"
	}
	for _, fn := range field.Names {
		speak(symbolToSpeech(fn.String()))
	}
	speak(as)
	speakExpr(field.Type)
	if field.Tag != nil {
		speak("with tag")
		speakExpr(field.Tag)
	}
}

func speakExpr(expr ast.Expr) {
	if expr == nil {
		return
	}
	switch v := expr.(type) {
	case *ast.Ident:
		speak(symbolToSpeech(v.String()))
	case *ast.ArrayType:
		if v.Len == nil {
			speak("slice of")
		} else {
			speakExpr(v.Len)
			speak("element")
			speak("array of")
		}
		speakExpr(v.Elt)
	case *ast.StarExpr:
		speak("pointer to")
		speakExpr(v.X)
	case *ast.MapType:
		speak("map with ")
		speakExpr(v.Key)
		speak("key and ")
		speakExpr(v.Value)
		speak("value")
	case *ast.SelectorExpr:
		speakExpr(v.X)
		speak("dot")
		speakExpr(v.Sel)
	case *ast.BinaryExpr:
		speakExpr(v.X)
		speakBinaryOp(v.Op.String())
		speakExpr(v.Y)
	case *ast.ParenExpr:
		speak("left paren")
		speakExpr(v.X)
		speak("right paren")
	case *ast.CallExpr:
		speakFunctionCall(v)
	case *ast.UnaryExpr:
		if v.Op.IsOperator() {
			speakUnaryOp(v.Op.String())
		}
		speakExpr(v.X)
	case *ast.BasicLit:
		speakString(v.Value)
	case *ast.SliceExpr:
		speak("slice")
		speakExpr(v.X)
		speak("from")
		if v.Low != nil {
			speakExpr(v.Low)
		} else {
			speak("start")
		}
		speak("to")
		if v.High != nil {
			speakExpr(v.High)
		} else {
			speak("end")
		}
		if v.Slice3 {
			speak("with cap ")
			speakExpr(v.Max)
		}
	case *ast.CompositeLit:
		speakCompositeLit(v)

	case *ast.KeyValueExpr:
		speakExpr(v.Key)
		speak("colon	")
		speakExpr(v.Value)

	case *ast.FuncLit:
		speak("lambda")
		speakFieldList(v.Type.Params, "taking", "parameter")
		speakFieldList(v.Type.Results, "and returning", "value")
		speakBlockStmt(v.Body, "is", "end lambda")

	case *ast.IndexExpr:
		speakExpr(v.X)
		speak("sub")
		speakExpr(v.Index)

	case *ast.InterfaceType:
		speakInterfaceType(v)

	case *ast.StructType:
		speakStructType(v)

	case *ast.TypeAssertExpr:
		speakExpr(v.X)
		speak("as type")
		speakExpr(v.Type)

	case *ast.ChanType:
		if v.Dir == ast.SEND {
			speak("send to channel")
			speakExpr(v.Value)
		} else {
			speak("received from channel")
			speakExpr(v.Value)
		}

	case *ast.Ellipsis:
		if v.Elt != nil {
			speak("variable number of")
			speakExpr(v.Elt)
		} else {
			speak("variable number")
		}

	case *ast.FuncType:
		speak("function")
		speakFieldList(v.Params, "taking ", "parameter")
		speakFieldList(v.Results, "and returning ", "value")

	case *ast.BadExpr:
		badDeclText := getFileString(v.From, v.To)
		speak("Bad expression")
		speakSymbol(badDeclText)
	}
}

func speakCompositeLit(c *ast.CompositeLit) {
	if len(c.Elts) == 0 {
		speak("empty")
	}
	if c.Type != nil {
		speakExpr(c.Type)
	}
	if len(c.Elts) > 0 {
		speak("containing")
	}
	first := true
	for _, e := range c.Elts {
		if !first {
			speak("comma")
		} else {
			first = false
		}
		speakExpr(e)
	}
}

func speakInterfaceType(iface *ast.InterfaceType) {
	if iface.Methods == nil || iface.Methods.List == nil || len(iface.Methods.List) == 0 {
		speak("empty interface")
	} else {
		speak("interface")
		speakFieldList(iface.Methods, "having", "method")
	}
}

func speakStructType(s *ast.StructType) {
	if s.Fields == nil || s.Fields.List == nil || len(s.Fields.List) == 0 {
		speak("empty struct")
	} else {
		speak("struct")
		speakFieldList(s.Fields, "having", "field")
	}
}

func speakFunctionCall(c *ast.CallExpr) {
	if len(c.Args) == 0 {
		speak("call")
	}
	speakExpr(c.Fun)
	if len(c.Args) > 0 {
		speak("of")
	}
	spokeEllipsis := false
	first := true
	for _, a := range c.Args {
		if !first {
			speak("comma	")
		} else {
			first = false
		}
		if c.Ellipsis != token.NoPos && c.Ellipsis < a.Pos() && !spokeEllipsis {
			speak("ellipsis")
			spokeEllipsis = true
		}
		speakExpr(a)
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

func speakBinaryOp(op string) {
	speechVal, ok := binaryOpSpeech[op]
	if ok {
		speak(speechVal)
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

func speakUnaryOp(op string) {
	speechVal, ok := unaryOpSpeech[op]
	if ok {
		speak(speechVal)
	}
}

func speakBlockStmt(stmts *ast.BlockStmt, bodyStart string, bodyEnd string) {
	speak(bodyStart)
	for _, bs := range stmts.List {
		speakStmt(bs)
	}
	speak(bodyEnd)
}

func speakStmt(stmt ast.Stmt) {
	switch v := stmt.(type) {
	case *ast.BlockStmt:
		speak("begin block")
		for _, bs := range v.List {
			speakStmt(bs)
		}
		speak("end block")
	case *ast.IfStmt:
		speakIfStatement(v)
	case *ast.ForStmt:
		speakForLoop(v)
	case *ast.RangeStmt:
		speak("range over ")
		speakExpr(v.X)
		speak("with")
		if v.Key != nil {
			speak("key")
			speakExpr(v.Key)
			if v.Value != nil {
				speak("and")
			}
		}
		if v.Value != nil {
			speak("value")
			speakExpr(v.Value)
		}
		if v.Body != nil {
			speakBlockStmt(v.Body, "range body", "end range")
		}
	case *ast.ReturnStmt:
		speak("return")
		first := true
		for _, e := range v.Results {
			if !first {
				speak("also")
			} else {
				first = false
			}
			speakExpr(e)
		}
	case *ast.AssignStmt:
		speakAssignStatement(v)

	case *ast.ExprStmt:
		speakExpr(v.X)

	case *ast.BranchStmt:
		speak(v.Tok.String())
		if v.Label != nil {
			speak("at")
			speakSymbol(v.Label.String())
		}
	case *ast.SwitchStmt:
		speakSwitchStatement(v)

	case *ast.TypeSwitchStmt:
		speakTypeSwitchStatement(v)

	case *ast.CommClause:
		speakCommClause(v)

	case *ast.CaseClause:
		speakSwitchCase(v)

	case *ast.DeferStmt:
		speak("defer")
		speakExpr(v.Call)

	case *ast.GoStmt:
		speak("go")
		speakExpr(v.Call)

	case *ast.EmptyStmt:
		speak("empty")

	case *ast.IncDecStmt:
		if v.Tok == token.INC {
			speak("increment")
		} else {
			speak("decrement")
		}
		speakExpr(v.X)

	case *ast.LabeledStmt:
		speak("label")
		speakSymbol(v.Label.String())
		speakStmt(v.Stmt)

	case *ast.SelectStmt:
		speakSelectStatement(v)

	case *ast.SendStmt:
		speak("send")
		speakExpr(v.Value)
		speak("to channel")
		speakExpr(v.Chan)

	case *ast.BadStmt:
		badDeclText := getFileString(v.From, v.To)
		speak("Bad statement")
		speakSymbol(badDeclText)

	case *ast.DeclStmt:
		speakDeclaration(v.Decl)
	}
}

func speakAssignStatement(s *ast.AssignStmt) {
	speak("let")
	if len(s.Lhs) > 1 && len(s.Lhs) == len(s.Rhs) {
		for i := range s.Lhs {
			speakExpr(s.Lhs[i])
			speak("equal")
			speakExpr(s.Rhs[i])
		}
	} else {
		first := true
		for _, l := range s.Lhs {
			if !first {
				speak("and")
			} else {
				first = false
			}
			speakExpr(l)
		}
		speak("equal")
		for _, r := range s.Rhs {
			speakExpr(r)
		}
	}
}
func speakIfStatement(s *ast.IfStmt) {
	speak("if")
	if s.Init != nil {
		speak("with initializer ")
		speakStmt(s.Init)
		speak("when")
	}
	if s.Cond != nil {
		speakExpr(s.Cond)
	}
	if s.Body != nil {
		bodyEnd := "end if"
		if s.Else != nil {
			bodyEnd = ""
		}
		speakBlockStmt(s.Body, "then", bodyEnd)
	}
	if s.Else != nil {
		switch e := s.Else.(type) {
		case *ast.BlockStmt:
			speakBlockStmt(e, "else", "end if")
		default:
			speakStmt(e)
		}
	}
}
func speakForLoop(fl *ast.ForStmt) {
	loopType := "for"
	if fl.Init == nil && fl.Post == nil {
		if fl.Cond == nil {
			speak("for ever")
		} else {
			speak("while")
			loopType = "while"
			speakExpr(fl.Cond)
		}
	} else {
		speak("for")
		if fl.Init == nil {
			speakStmt(fl.Init)
		}
		if fl.Cond != nil {
			speak("while")
			speakExpr(fl.Cond)
		}
		if fl.Post != nil {
			speakStmt(fl.Post)
		}
	}
	speakBlockStmt(fl.Body, "do", "end "+loopType+" loop")
}

func speakSwitchStatement(s *ast.SwitchStmt) {
	speak("switch")
	if s.Init != nil {
		speak("with initializer")
		speakStmt(s.Init)
	}
	speak("on")
	speakExpr(s.Tag)
	speakBlockStmt(s.Body, "", "end switch")

}

func speakTypeSwitchStatement(s *ast.TypeSwitchStmt) {
	speak("switch")
	if s.Init != nil {
		speak("with initializer")
		speakStmt(s.Init)
	}

	speak("on type")
	speakStmt(s.Assign)
	speakBlockStmt(s.Body, "", "end type switch")

}

func speakCommClause(c *ast.CommClause) {
	if c.Comm != nil {
		speak("default")
	} else {
		speak("case")
	}
	speakStmt(c.Comm)
	for _, cs := range c.Body {
		speakStmt(cs)
	}
}

func speakSwitchCase(c *ast.CaseClause) {
	if len(c.List) == 0 {
		speak("default")
	} else {
		speak("case")
	}
	first := true
	for _, e := range c.List {
		if !first {
			speak("or")
		} else {
			first = false
		}
		speakExpr(e)
	}
	for _, cs := range c.Body {
		speakStmt(cs)
	}
}

func speakSelectStatement(s *ast.SelectStmt) {
	speak("select")
	speakBlockStmt(s.Body, "", "end select")

}
