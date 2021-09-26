package formatter

import (
	"github.com/google/go-jsonnet/ast"
	"github.com/jdbaldry/jsonnet-tool/internal/go-jsonnet/pass"
)

// capturer captures referenceable parts of the AST that can be used in
// an expanded form.
type capturer struct {
	pass.Base
	Locals map[ast.Identifier]ast.Node
}

func NewCapturer() *capturer {
	return &capturer{Locals: make(map[ast.Identifier]ast.Node)}
}

func (c *capturer) Visit(p pass.ASTPass, node *ast.Node, ctx pass.Context) {
	if local, ok := (*node).(*ast.Local); ok {
		for _, bind := range local.Binds {
			body := bind.Body
			if imp, ok := body.(*ast.Import); ok {
				body = &ast.Parens{Inner: imp}
			}
			c.Locals[bind.Variable] = body
		}
	}
	c.Base.Visit(p, node, ctx)
}

// expander expands variables using captured parts of the AST.
type expander struct {
	pass.Base
	Locals map[ast.Identifier]ast.Node
}

func NewExpanderFromCapturer(c *capturer) *expander {
	return &expander{Locals: c.Locals}
}

func (e *expander) Visit(p pass.ASTPass, node *ast.Node, ctx pass.Context) {
	variable, ok := (*node).(*ast.Var)
	if ok {
		if expanded, ok := e.Locals[variable.Id]; ok {
			*node = expanded
		}
	}
	e.Base.Visit(p, node, ctx)
}
