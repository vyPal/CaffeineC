package parser

import (
	"strconv"

	"github.com/alecthomas/participle/v2/lexer"
)

type Bool bool

func (b *Bool) Capture(values []string) error {
	*b = values[0] == "true"
	return nil
}

type Duration struct {
	Number float64
	Unit   string
}

func (d *Duration) Capture(values []string) error {
	num, err := strconv.ParseFloat(values[0], 64)
	if err != nil {
		return err
	}
	d.Number = num
	d.Unit = values[1]
	return nil
}

type Value struct {
	Pos      lexer.Position
	Float    *float64  `parser:"  @Float"`
	Duration *Duration `parser:"| @Int @('h' | 'm' | 's' | 'ms' | 'us' | 'ns')"`
	Int      *int64    `parser:"| @('-'? Int)"`
	Bool     *Bool     `parser:"| @('true' | 'false')"`
	String   *string   `parser:"| @String"`
	Null     bool      `parser:"| @'null'"`
}

type Identifier struct {
	Pos  lexer.Position
	Name string      `parser:"@Ident"`
	GEP  *Expression `parser:"('[' @@ ']')?"`
	Sub  *Identifier `parser:"( '.' @@ )*"`
}

type ArgumentList struct {
	Pos       lexer.Position
	Arguments []*Expression `parser:"( @@ ( ',' @@ )* )?"`
}

type ClassInitializer struct {
	Pos       lexer.Position
	ClassName string       `parser:"@Ident"`
	Args      ArgumentList `parser:"'(' @@ ')'"`
}

type FunctionCall struct {
	Pos          lexer.Position
	FunctionName string       `parser:"@( Ident | String )"`
	Args         ArgumentList `parser:"'(' @@ ')'"`
}

type Factor struct {
	Pos              lexer.Position
	Value            *Value            `parser:"  @@"`
	FunctionCall     *FunctionCall     `parser:"| (?= ( Ident | String ) '(') @@"`
	BitCast          *BitCast          `parser:"| '(' @@?"`
	ClassInitializer *ClassInitializer `parser:"| 'new' @@"`
	SubExpression    *Expression       `parser:"| '(' @@ ')'"`
	ClassMethod      *ClassMethod      `parser:"| (?= Ident ( '.' Ident)+ '(') @@"`
	Identifier       *Identifier       `parser:"| @@"`
}

type Term struct {
	Pos   lexer.Position
	Left  *Factor   `parser:"@@"`
	Right []*OpTerm `parser:"@@*"`
}

type OpTerm struct {
	Pos  lexer.Position
	Op   string  `parser:"@( '*' | '/' | '%' )"`
	Term *Factor `parser:"@@"`
}

type Comparison struct {
	Pos   lexer.Position
	Left  *Term           `parser:"@@"`
	Right []*OpComparison `parser:"@@*"`
}

type OpComparison struct {
	Pos        lexer.Position
	Op         string `parser:"@( ('=' '=') | ( '<' '=' ) | '<'  | ( '>' '=' ) |'>' | ('!' '=') )"`
	Comparison *Term  `parser:"@@"`
}

type Expression struct {
	Pos   lexer.Position
	Left  *Comparison     `parser:"@@"`
	Right []*OpExpression `parser:"@@*"`
}

type OpExpression struct {
	Pos        lexer.Position
	Op         string      `parser:"@( '+' | '-' )"`
	Expression *Comparison `parser:"@@"`
}

type Assignment struct {
	Pos   lexer.Position
	Left  *Identifier `parser:"@@"`
	Right *Expression `parser:"'=' @@"`
}

type VariableDefinition struct {
	Pos        lexer.Position
	Constant   bool        `parser:"'const'?"`
	Name       string      `parser:"'var' @Ident"`
	Type       string      `parser:"':' @('*'* Ident)"`
	Assignment *Expression `parser:"( '=' @@ )?"`
}

type FieldDefinition struct {
	Pos     lexer.Position
	Private bool   `parser:"@'private'?"`
	Name    string `parser:"@Ident"`
	Type    string `parser:"':' @('*'* Ident) ';'"`
}

type ArgumentDefinition struct {
	Pos  lexer.Position
	Name string `parser:"@Ident"`
	Type string `parser:"':' @('*'* Ident)"`
}

type FuncName struct {
	Dummy  string `parser:"'func'"`
	Op     bool   `parser:"@'op'?"`
	Get    bool   `parser:"@'get'?"`
	Set    bool   `parser:"@'set'?"`
	Name   string `parser:"@Ident?"`
	String string `parser:"@String?"`
}

type FunctionDefinition struct {
	Pos        lexer.Position
	Private    bool                  `parser:"@'private'?"`
	Static     bool                  `parser:"@'static'?"`
	Variadic   bool                  `parser:"@'vararg'?"`
	Name       FuncName              `parser:"@@"`
	Parameters []*ArgumentDefinition `parser:"'(' ( @@ ( ',' @@ )* )? ')'"`
	ReturnType string                `parser:"( ':' @('*'* Ident) )?"`
	Body       []*Statement          `parser:"'{' @@* '}'"`
}

type ClassDefinition struct {
	Pos  lexer.Position
	Name string       `parser:"@Ident"`
	Body []*Statement `parser:"'{' @@* '}'"`
}

type ClassMethod struct {
	Pos        lexer.Position
	Identifier *Identifier   `parser:"@@"`
	Args       *ArgumentList `parser:"'(' @@ ')'"`
}

type If struct {
	Pos       lexer.Position
	Condition *Expression  `parser:"'(' @@ ')'"`
	Body      []*Statement `parser:"'{' @@* '}'"`
	ElseIf    []*ElseIf    `parser:"( 'else' 'if' @@ )*"`
	Else      []*Statement `parser:"( 'else' '{' @@* '}' )?"`
}

type ElseIf struct {
	Pos       lexer.Position
	Condition *Expression  `parser:"'(' @@ ')'"`
	Body      []*Statement `parser:"'{' @@* '}'"`
}

type For struct {
	Pos         lexer.Position
	Initializer *Statement   `parser:"'(' @@"`
	Condition   *Expression  `parser:"@@ ';'"`
	Increment   *Statement   `parser:"@@ ')'"`
	Body        []*Statement `parser:"'{' @@* '}'"`
}

type While struct {
	Pos       lexer.Position
	Condition *Expression  `parser:"'(' @@ ')'"`
	Body      []*Statement `parser:"'{' @@* '}'"`
}

type Return struct {
	Pos        lexer.Position
	Expression *Expression `parser:"@@? ';'"`
}

type ExternalFunctionDefinition struct {
	Pos        lexer.Position
	Variadic   bool                  `parser:"@'vararg'?"`
	Name       string                `parser:"'func' @( Ident | String )"`
	Parameters []*ArgumentDefinition `parser:"'(' ( @@ ( ',' @@ )* )? ')'"`
	ReturnType string                `parser:"( ':' @('*'* Ident) )?"`
}

type Import struct {
	Package string `parser:"@String ';'"`
}

type FromImport struct {
	Package string `parser:"'from' @String 'import'"`
	Symbol  string `parser:"@Ident"`
	Alias   string `parser:"('as' @Ident)? ';'"`
}

type FromImportMultiple struct {
	Package string   `parser:"'from' @String 'import' '{'"`
	Symbols []Symbol `parser:"@@ (',' @@)* '}' ';'"`
}

type Symbol struct {
	Name  string `parser:"@Ident"`
	Alias string `parser:"('as' @Ident)?"`
}

type BitCast struct {
	Pos  lexer.Position
	Expr *Expression `parser:"@@ ')'"`
	Type string      `parser:"':' @('*'* Ident)"`
}

type Statement struct {
	Pos                lexer.Position
	VariableDefinition *VariableDefinition         `parser:"(?= 'const'? 'var' Ident) @@? (';' | '\\n')?"`
	Assignment         *Assignment                 `parser:"| (?= Ident ( '[' ~']' ']' )? ( '.' Ident ( '[' ~']' ']' )? )* '=') @@? (';' | '\\n')?"`
	External           *ExternalFunctionDefinition `parser:"| 'extern' @@ ';'"`
	Export             *Statement                  `parser:"| 'export' @@"`
	FunctionDefinition *FunctionDefinition         `parser:"| (?= 'private'? 'static'? 'func') @@?"`
	ClassDefinition    *ClassDefinition            `parser:"| 'class' @@?"`
	If                 *If                         `parser:"| 'if' @@?"`
	For                *For                        `parser:"| 'for' @@?"`
	While              *While                      `parser:"| 'while' @@?"`
	Return             *Return                     `parser:"| 'return' @@?"`
	FieldDefinition    *FieldDefinition            `parser:"| (?= 'private'? Ident ':' '*'* Ident) @@?"`
	Import             *Import                     `parser:"| 'import' @@?"`
	FromImportMultiple *FromImportMultiple         `parser:"| (?= 'from' String 'import' '{') @@?"`
	FromImport         *FromImport                 `parser:"| (?= 'from' String 'import') @@?"`
	Break              *string                     `parser:"| @('break' (';' | '\\n')?)"`
	Continue           *string                     `parser:"| @('continue' (';' | '\\n')?)"`
	Comment            *string                     `parser:"| @Comment"`
	Expression         *Expression                 `parser:"| @@ ';'"`
}

type Program struct {
	Pos        lexer.Position
	Package    string       `parser:"'package' @Ident ';'"`
	Statements []*Statement `parser:"@@*"`
}
