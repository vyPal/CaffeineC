package parser

import (
	"errors"

	"github.com/alecthomas/participle/v2/lexer"
)

type Bool bool

func (b *Bool) Capture(values []string) error {
	switch values[0] {
	case "true", "True":
		*b = true
		return nil
	case "false", "False":
		*b = false
		return nil
	default:
		return errors.New(values[0] + " is not a valid boolean value")
	}
}

type Value struct {
	Pos    lexer.Position
	Array  []*Expression `parser:"'[' ( @@ ( ',' @@ )* )? ']'"`
	Float  *float64      `parser:"  @('-'? Float)"`
	Int    *int64        `parser:"| @('-'? Int)"`
	HexInt *string       `parser:"| @('-'? '0x' (Int | 'a' | 'b' | 'c' | 'd' | 'e' | 'f' | 'A' | 'B' | 'C' | 'D' | 'E' | 'F')+)"`
	Bool   *Bool         `parser:"| @('true' | 'True' | 'false' | 'False')"`
	String *string       `parser:"| @String"`
	Null   bool          `parser:"| @'null'"`
}

type Identifier struct {
	Pos   lexer.Position
	Ref   string      `parser:"@'&'*"`
	Deref string      `parser:"@'*'*"`
	Name  string      `parser:"@Ident"`
	GEP   *Expression `parser:"('[' @@ ']')?"`
	Sub   *Identifier `parser:"( '.' @@ )*"`
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
	Unpack           bool              `parser:"@'...'?"`
	Value            *Value            `parser:"  @@"`
	FunctionCall     *FunctionCall     `parser:"| (?= ( Ident | String ) '(') @@"`
	BitCast          *BitCast          `parser:"| '(' @@"`
	ClassInitializer *ClassInitializer `parser:"| 'new' @@"`
	ClassMethod      *ClassMethod      `parser:"| (?= Ident ( '.' Ident)+ '(') @@"`
	Identifier       *Identifier       `parser:"| @@"`
}

type BitCast struct {
	Pos  lexer.Position
	Expr *Expression `parser:"@@ ')'"`
	Type *Type       `parser:"(':' @@)?"`
}

type Assignment struct {
	Pos    lexer.Position
	Idents []*Identifier `parser:"@@ ( ',' @@ )*"`
	Op     string        `parser:"@('=' | '+=' | '-=' | '*=' | '/=' | '%=' | '&=' | '|=' | '^=' | '<<=' | '>>=' | '>>>=' | '??=')"`
	Right  *Expression   `parser:"@@"`
}

type Expression struct {
	Pos       lexer.Position
	Condition *LogicalOr  `parser:"@@"`
	True      *Expression `parser:"'?' @@ ':'"`
	False     *Expression `parser:"| @@"`
}

type LogicalOr struct {
	Pos   lexer.Position
	Left  *LogicalAnd  `parser:"@@"`
	Op    string       `parser:"@( '||' | 'or' )"`
	Right []*LogicalOr `parser:"@@"`
}

type LogicalAnd struct {
	Pos   lexer.Position
	Left  *BitwiseOr    `parser:"@@"`
	Op    string        `parser:"@( '&&' | 'and' )"`
	Right []*LogicalAnd `parser:"@@"`
}

type BitwiseOr struct {
	Pos   lexer.Position
	Left  *BitwiseXor  `parser:"@@"`
	Op    string       `parser:"@'|'"`
	Right []*BitwiseOr `parser:"@@"`
}

type BitwiseXor struct {
	Pos   lexer.Position
	Left  *BitwiseAnd   `parser:"@@"`
	Op    string        `parser:"@'^'"`
	Right []*BitwiseXor `parser:"@@"`
}

type BitwiseAnd struct {
	Pos   lexer.Position
	Left  *Equality     `parser:"@@"`
	Op    string        `parser:"@'&'"`
	Right []*BitwiseAnd `parser:"@@"`
}

type Equality struct {
	Pos   lexer.Position
	Left  *Relational `parser:"@@"`
	Op    string      `parser:"@( '==' | '!=' )"`
	Right []*Equality `parser:"@@"`
}

type Relational struct {
	Pos   lexer.Position
	Left  *Shift        `parser:"@@"`
	Op    string        `parser:"@( '<=' | '>=' | '<' | '>' )"`
	Right []*Relational `parser:"@@"`
}

type Shift struct {
	Pos   lexer.Position
	Left  *Additive `parser:"@@"`
	Op    string    `parser:"@( '<<' | '>>' | '>>>' )"`
	Right []*Shift  `parser:"@@"`
}

type Additive struct {
	Pos   lexer.Position
	Left  *Multiplicative `parser:"@@"`
	Op    string          `parser:"@( '+' | '-' )"`
	Right []*Additive     `parser:"@@"`
}

type Multiplicative struct {
	Pos   lexer.Position
	Left  *LogicalNot       `parser:"@@"`
	Op    string            `parser:"@( '*' | '/' | '%' )"`
	Right []*Multiplicative `parser:"@@"`
}

type LogicalNot struct {
	Pos   lexer.Position
	Op    string      `parser:"@'!'"`
	Right *BitwiseNot `parser:"@@"`
}

type BitwiseNot struct {
	Pos   lexer.Position
	Op    string          `parser:"@'~'"`
	Right *PrefixAdditive `parser:"@@"`
}

type PrefixAdditive struct {
	Pos   lexer.Position
	Op    string           `parser:"@('++' | '--')"`
	Right *PostfixAdditive `parser:"@@"`
}

type PostfixAdditive struct {
	Pos  lexer.Position
	Left *Factor `parser:"@@"`
	Op   string  `parser:"@('++' | '--')"`
}

type VariableDefinition struct {
	Pos        lexer.Position
	Constant   string      `parser:"@('const' | 'var')"`
	Name       string      `parser:"@Ident"`
	Type       *Type       `parser:"':' @@"`
	Assignment *Expression `parser:"( '=' @@ )?"`
}

type FieldDefinition struct {
	Pos     lexer.Position
	Private bool   `parser:"@'private'?"`
	Name    string `parser:"@Ident"`
	Type    *Type  `parser:"':' @@ ';'"`
}

type ArgumentDefinition struct {
	Pos  lexer.Position
	Name string `parser:"@Ident"`
	Type *Type  `parser:"':' @@"`
}

type FuncName struct {
	Dummy string `parser:"'func'"`
	Op    bool   `parser:"@'op'?"`
	Get   bool   `parser:"@'get'?"`
	Set   bool   `parser:"@'set'?"`
	Name  string `parser:"@(Ident | String)"`
}

type FunctionDefinition struct {
	Pos        lexer.Position
	Private    bool                  `parser:"@'private'?"`
	Static     bool                  `parser:"@'static'?"`
	Name       FuncName              `parser:"@@"`
	Parameters []*ArgumentDefinition `parser:"'(' ( @@ ( ',' @@ )* )?"`
	Variadic   string                `parser:"(',' '...' @Ident)?"`
	ReturnType []*Type               `parser:"')' ( ':' @@ ( ',' @@ )* )?"`
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

type Until struct {
	Pos       lexer.Position
	Condition *Expression  `parser:"'(' @@ ')'"`
	Body      []*Statement `parser:"'{' @@* '}'"`
}

type Switch struct {
	Pos       lexer.Position
	Condition *Expression  `parser:"'(' @@ ')'"`
	Cases     []*Case      `parser:"'{' @@*"`
	Default   []*Statement `parser:"('default' ':' @@*)? '}'"`
}

type Case struct {
	Pos    lexer.Position
	Values []*Expression `parser:"('case' @@ ( ',' @@ )* ':'"`
	Body   []*Statement  `parser:"@@*"`
}

type Return struct {
	Pos         lexer.Position
	Expressions []*Expression `parser:"@@ ( ',' @@ )* ';'"`
}

type ExternalFunctionDefinition struct {
	Pos        lexer.Position
	Name       string                `parser:"'func' @( Ident | String )"`
	Parameters []*ArgumentDefinition `parser:"'(' ( @@ ( ',' @@ )* )?"`
	Variadic   bool                  `parser:"@(',' '...')?"`
	ReturnType []*Type               `parser:"')' ( ':' @@ ( ',' @@ )* )?"`
}

type TryCatch struct {
	Pos   lexer.Position
	Try   []*Statement `parser:"'try' '{' @@* '}'"`
	Catch *Catch       `parser:"'catch' @@"`
	Final []*Statement `parser:"('finally' '{' @@* '}')?"`
}

type Catch struct {
	Pos  lexer.Position
	Name string       `parser:"@Ident"`
	Body []*Statement `parser:"'{' @@* '}'"`
}

type Type struct {
	Pos   lexer.Position
	Array *Expression `parser:"('[' @@ ']')?"`
	Ptr   string      `parser:"'*'*"`
	Inner *Type       `parser:"@@ "`
	Name  string      `parser:"| @Ident"`
}

type Import struct {
	Package string `parser:"@String"`
	Alias   string `parser:"('as' @Ident)? ';'"`
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

type Statement struct {
	Pos                lexer.Position
	VariableDefinition *VariableDefinition         `parser:"(?= ('const' | 'var') Ident) @@? (';' | '\\n')?"`
	Assignment         *Assignment                 `parser:"| (?= Ident ( '[' ~']' ']' )? ( '.' Ident ( '[' ~']' ']' )? )* (',' Ident ( '[' ~']' ']' )? ( '.' Ident ( '[' ~']' ']' )? )*)* '=') @@? (';' | '\\n')?"`
	External           *ExternalFunctionDefinition `parser:"| 'extern' @@ ';'"`
	Export             *Statement                  `parser:"| 'export' @@"`
	FunctionDefinition *FunctionDefinition         `parser:"| (?= 'private'? 'static'? 'func') @@?"`
	TryCatch           *TryCatch                   `parser:"| 'try' @@"`
	Switch             *Switch                     `parser:"| 'switch' @@"`
	ClassDefinition    *ClassDefinition            `parser:"| 'class' @@?"`
	If                 *If                         `parser:"| 'if' @@?"`
	For                *For                        `parser:"| 'for' @@?"`
	While              *While                      `parser:"| 'while' @@?"`
	Until              *Until                      `parser:"| 'until' @@?"`
	Return             *Return                     `parser:"| 'return' @@?"`
	FieldDefinition    *FieldDefinition            `parser:"| (?= 'private'? Ident ':' ('[' ~']' ']')? '*'* Ident) @@?"`
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
