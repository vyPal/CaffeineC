package parser

import "strconv"

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
	Float    *float64  `parser:"  @Float"`
	Int      *int64    `parser:"| @Int"`
	Bool     *Bool     `parser:"| @('true' | 'false')"`
	String   *string   `parser:"| @String"`
	Duration *Duration `parser:"| @Int @('h' | 'm' | 's' | 'ms' | 'us' | 'ns')"`
}

type Identifier struct {
	Name string      `parser:"@Ident"`
	Sub  *Identifier `parser:"( '.' @@ )*"`
}

type ArgumentList struct {
	Arguments []*Expression `parser:"( @@ ( ',' @@ )* )?"`
}

type ClassInitializer struct {
	ClassName string       `parser:"'new' @Ident"`
	Args      ArgumentList `parser:"'(' @@ ')' ';'"`
}

type FunctionCall struct {
	FunctionName string       `parser:"@Ident"`
	Args         ArgumentList `parser:"'(' @@ ')' ';'"`
}

type Factor struct {
	Value            *Value            `parser:"  @@"`
	ClassInitializer *ClassInitializer `parser:"| (?= 'new') @@"`
	SubExpression    *Expression       `parser:"| '(' @@ ')'"`
	FunctionCall     *FunctionCall     `parser:"| (?= Ident '(') @@"`
	ClassMethod      *ClassMethod      `parser:"| (?= Ident ( '.' Ident)+ '(') @@"`
	Identifier       *Identifier       `parser:"| @@"`
}

type Term struct {
	Left  *Factor   `parser:"@@"`
	Right []*OpTerm `parser:"@@*"`
}

type OpTerm struct {
	Op   string  `parser:"@( '*' | '/' | '%' )"`
	Term *Factor `parser:"@@"`
}

type Comparison struct {
	Left  *Term           `parser:"@@"`
	Right []*OpComparison `parser:"@@*"`
}

type OpComparison struct {
	Op         string `parser:"@( ('=' '=') | ( '<' '=' ) | '<'  | ( '>' '=' ) |'>' | ('!' '=') )"`
	Comparison *Term  `parser:"@@"`
}

type Expression struct {
	Left  *Comparison     `parser:"@@"`
	Right []*OpExpression `parser:"@@*"`
}

type OpExpression struct {
	Op         string      `parser:"@( '+' | '-' )"`
	Expression *Comparison `parser:"@@"`
}

type Assignment struct {
	Left  *Identifier `parser:"@@"`
	Right *Expression `parser:"'=' @@"`
}

type VariableDefinition struct {
	Name       string      `parser:"'var' @Ident"`
	Type       string      `parser:"':' @Ident"`
	Assignment *Expression `parser:"( '=' @@ )?"`
}

type FieldDefinition struct {
	Private bool   `parser:"@'private'?"`
	Name    string `parser:"@Ident"`
	Type    string `parser:"':' @Ident ';'"`
}

type ArgumentDefinition struct {
	Name string `parser:"@Ident"`
	Type string `parser:"':' @Ident"`
}

type FunctionDefinition struct {
	Private    bool                  `parser:"@'private'?"`
	Static     bool                  `parser:"@'static'?"`
	Name       string                `parser:"'func' @Ident"`
	Parameters []*ArgumentDefinition `parser:"'(' ( @@ ( ',' @@ )* )? ')'"`
	ReturnType string                `parser:"( ':' @Ident )?"`
	Body       []*Statement          `parser:"'{' @@* '}'"`
}

type ClassDefinition struct {
	Name string       `parser:"'class' @Ident"`
	Body []*Statement `parser:"'{' @@* '}'"`
}

type ClassMethod struct {
	Identifier *Identifier   `parser:"@@"`
	Args       *ArgumentList `parser:"'(' @@ ')' ';'"`
}

type If struct {
	Condition *Expression  `parser:"'if' '(' @@ ')'"`
	Body      []*Statement `parser:"'{' @@* '}'"`
	ElseIf    []*ElseIf    `parser:"( 'else' 'if' @@ )*"`
	Else      []*Statement `parser:"( 'else' '{' @@* '}' )?"`
}

type ElseIf struct {
	Condition *Expression  `parser:"'else' 'if' '(' @@ ')'"`
	Body      []*Statement `parser:"'{' @@* '}'"`
}

type For struct {
	Initializer *VariableDefinition `parser:"'for' '(' @@ ';'"`
	Condition   *Expression         `parser:"@@ ';'"`
	Increment   *Assignment         `parser:"@@ ')'"`
	Body        []*Statement        `parser:"'{' @@* '}'"`
}

type While struct {
	Condition *Expression  `parser:"'while' '(' @@ ')'"`
	Body      []*Statement `parser:"'{' @@* '}'"`
}

type Return struct {
	Expression *Expression `parser:"'return' @@ ';'"`
}

type ExternalFunctionDefinition struct {
	Name       string                `parser:"'extern' 'func' @Ident"`
	Parameters []*ArgumentDefinition `parser:"'(' ( @@ ( ',' @@ )* )? ')'"`
	ReturnType string                `parser:"( ':' @('*'? Ident) )?"`
}

type Statement struct {
	VariableDefinition *VariableDefinition         `parser:"(?= 'var' Ident) @@? ';'"`
	Assignment         *Assignment                 `parser:"| (?= Ident '=') @@? ';'"`
	ExternalFunction   *ExternalFunctionDefinition `parser:"| (?= 'extern' 'func') @@? ';'"`
	FunctionDefinition *FunctionDefinition         `parser:"| (?= 'private'? 'static'? 'func') @@?"`
	ClassDefinition    *ClassDefinition            `parser:"| (?= 'class') @@?"`
	If                 *If                         `parser:"| (?= 'if') @@?"`
	For                *For                        `parser:"| (?= 'for') @@?"`
	While              *While                      `parser:"| (?= 'while') @@?"`
	Return             *Return                     `parser:"| (?= 'return') @@?"`
	FieldDefinition    *FieldDefinition            `parser:"| (?= 'private'? Ident ':' Ident) @@?"`
	Break              *string                     `parser:"| 'break' ';'"`
	Continue           *string                     `parser:"| 'continue' ';'"`
	Expression         *Expression                 `parser:"| @@"`
}

type Program struct {
	Statements []*Statement `parser:"@@*"`
}