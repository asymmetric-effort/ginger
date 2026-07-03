package config

// TokenType identifies the type of a lexer token.
type TokenType int

const (
	tokenEOF TokenType = iota
	tokenScalar
	tokenMappingKey
	tokenSequenceEntry
	tokenBlockEnd
	tokenDocumentStart
	tokenDocumentEnd
	tokenError
)

// Token is a single lexer token with position information.
type Token struct {
	Type   TokenType
	Value  string
	Line   int
	Column int
	Indent int
}
