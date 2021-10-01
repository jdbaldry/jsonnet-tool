/*
Copyright 2019 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package formatter

import (
	"bytes"
	"fmt"

	"github.com/google/go-jsonnet/ast"
)

// Unparser can unparse the Jsonnet AST back into an expression.
type Unparser struct {
	buf     bytes.Buffer
	options Options
}

func (u *Unparser) Write(str string) {
	u.buf.WriteString(str)
}

// Fill Pretty-prints fodder.
// The crowded and separateToken params control whether single whitespace
// characters are added to keep tokens from joining together in the output.
// The intuition of crowded is that the caller passes true for crowded if the
// last thing printed would crowd whatever we're printing here.  For example, if
// we just printed a ',' then crowded would be true.  If we just printed a '('
// then crowded would be false because we don't want the space after the '('.
//
// If crowded is true, a space is printed after any fodder, unless
// separateToken is false or the fodder ended with a newline.
// If crowded is true and separateToken is false and the fodder begins with
// an interstitial, then the interstitial is prefixed with a single space, but
// there is no space after the interstitial.
// If crowded is false and separateToken is true then a space character
// is only printed when the fodder ended with an interstitial comment (which
// creates a crowded situation where there was not one before).
// If crowded is false and separateToken is false then no space is printed
// after or before the fodder, even if the last fodder was an interstitial.
func (u *Unparser) Fill(fodder ast.Fodder, crowded bool, separateToken bool) {
	var lastIndent int
	for _, fod := range fodder {
		switch fod.Kind {
		case ast.FodderParagraph:
			for i, l := range fod.Comment {
				// Do not indent empty lines (note: first line is never empty).
				if len(l) > 0 {
					// First line is already indented by previous fod.
					if i > 0 {
						for i := 0; i < lastIndent; i++ {
							u.Write(" ")
						}
					}
					u.Write(l)
				}
				u.Write("\n")
			}
			for i := 0; i < fod.Blanks; i++ {
				u.Write("\n")
			}
			for i := 0; i < fod.Indent; i++ {
				u.Write(" ")
			}
			lastIndent = fod.Indent
			crowded = false

		case ast.FodderLineEnd:
			if len(fod.Comment) > 0 {
				u.Write("  ")
				u.Write(fod.Comment[0])
			}
			for i := 0; i <= fod.Blanks; i++ {
				u.Write("\n")
			}
			for i := 0; i < fod.Indent; i++ {
				u.Write(" ")
			}
			lastIndent = fod.Indent
			crowded = false

		case ast.FodderInterstitial:
			if crowded {
				u.Write(" ")
			}
			u.Write(fod.Comment[0])
			crowded = true
		}
	}
	if separateToken && crowded {
		u.Write(" ")
	}
}

func (u *Unparser) unparseSpecs(spec *ast.ForSpec) {
	if spec.Outer != nil {
		u.unparseSpecs(spec.Outer)
	}
	u.Fill(spec.ForFodder, true, true)
	u.Write("for")
	u.Fill(spec.VarFodder, true, true)
	u.Write(string(spec.VarName))
	u.Fill(spec.InFodder, true, true)
	u.Write("in")
	u.Unparse(spec.Expr, true)
	for _, cond := range spec.Conditions {
		u.Fill(cond.IfFodder, true, true)
		u.Write("if")
		u.Unparse(cond.Expr, true)
	}
}

func (u *Unparser) unparseParams(fodderL ast.Fodder, params []ast.Parameter, trailingComma bool, fodderR ast.Fodder) {
	u.Fill(fodderL, false, false)
	u.Write("(")
	first := true
	for _, param := range params {
		if !first {
			u.Write(",")
		}
		u.Fill(param.NameFodder, !first, true)
		u.unparseID(param.Name)
		if param.DefaultArg != nil {
			u.Fill(param.EqFodder, false, false)
			u.Write("=")
			u.Unparse(param.DefaultArg, false)
		}
		u.Fill(param.CommaFodder, false, false)
		first = false
	}
	if trailingComma {
		u.Write(",")
	}
	u.Fill(fodderR, false, false)
	u.Write(")")
}

func (u *Unparser) unparseFieldParams(field ast.ObjectField) {
	m := field.Method
	if m != nil {
		u.unparseParams(m.ParenLeftFodder, m.Parameters, m.TrailingComma,
			m.ParenRightFodder)
	}
}

func (u *Unparser) unparseFields(fields ast.ObjectFields, crowded bool) {
	first := true
	for _, field := range fields {
		if !first {
			u.Write(",")
		}

		// An aux function so we don't repeat ourselves for the 3 kinds of
		// basic field.
		unparseFieldRemainder := func(field ast.ObjectField) {
			u.unparseFieldParams(field)
			u.Fill(field.OpFodder, false, false)
			if field.SuperSugar {
				u.Write("+")
			}
			switch field.Hide {
			case ast.ObjectFieldInherit:
				u.Write(":")
			case ast.ObjectFieldHidden:
				u.Write("::")
			case ast.ObjectFieldVisible:
				u.Write(":::")
			}
			u.Unparse(field.Expr2, true)
		}

		switch field.Kind {
		case ast.ObjectLocal:
			u.Fill(field.Fodder1, !first || crowded, true)
			u.Write("local")
			u.Fill(field.Fodder2, true, true)
			u.unparseID(*field.Id)
			u.unparseFieldParams(field)
			u.Fill(field.OpFodder, true, true)
			u.Write("=")
			u.Unparse(field.Expr2, true)

		case ast.ObjectFieldID:
			u.Fill(field.Fodder1, !first || crowded, true)
			u.unparseID(*field.Id)
			unparseFieldRemainder(field)

		case ast.ObjectFieldStr:
			u.Unparse(field.Expr1, !first || crowded)
			unparseFieldRemainder(field)

		case ast.ObjectFieldExpr:
			u.Fill(field.Fodder1, !first || crowded, true)
			u.Write("[")
			u.Unparse(field.Expr1, false)
			u.Fill(field.Fodder2, false, false)
			u.Write("]")
			unparseFieldRemainder(field)

		case ast.ObjectAssert:
			u.Fill(field.Fodder1, !first || crowded, true)
			u.Write("assert")
			u.Unparse(field.Expr2, true)
			if field.Expr3 != nil {
				u.Fill(field.OpFodder, true, true)
				u.Write(":")
				u.Unparse(field.Expr3, true)
			}
		}

		first = false
		u.Fill(field.CommaFodder, false, false)
	}

}

func (u *Unparser) unparseID(id ast.Identifier) {
	u.Write(string(id))
}

func (u *Unparser) Unparse(expr ast.Node, crowded bool) {

	if leftRecursive(expr) == nil {
		u.Fill(*expr.OpenFodder(), crowded, true)
	}

	switch node := expr.(type) {
	case *ast.Apply:
		u.Unparse(node.Target, crowded)
		u.Fill(node.FodderLeft, false, false)
		u.Write("(")
		first := true
		for _, arg := range node.Arguments.Positional {
			if !first {
				u.Write(",")
			}
			space := !first
			u.Unparse(arg.Expr, space)
			u.Fill(arg.CommaFodder, false, false)
			first = false
		}
		for _, arg := range node.Arguments.Named {
			if !first {
				u.Write(",")
			}
			space := !first
			u.Fill(arg.NameFodder, space, true)
			u.unparseID(arg.Name)
			space = false
			u.Write("=")
			u.Unparse(arg.Arg, space)
			u.Fill(arg.CommaFodder, false, false)
			first = false
		}
		if node.TrailingComma {
			u.Write(",")
		}
		u.Fill(node.FodderRight, false, false)
		u.Write(")")
		if node.TailStrict {
			u.Fill(node.TailStrictFodder, true, true)
			u.Write("tailstrict")
		}

	case *ast.ApplyBrace:
		u.Unparse(node.Left, crowded)
		u.Unparse(node.Right, true)

	case *ast.Array:
		u.Write("[")
		first := true
		for _, element := range node.Elements {
			if !first {
				u.Write(",")
			}
			u.Unparse(element.Expr, !first || u.options.PadArrays)
			u.Fill(element.CommaFodder, false, false)
			first = false
		}
		if node.TrailingComma {
			u.Write(",")
		}
		u.Fill(node.CloseFodder, len(node.Elements) > 0, u.options.PadArrays)
		u.Write("]")

	case *ast.ArrayComp:
		u.Write("[")
		u.Unparse(node.Body, u.options.PadArrays)
		u.Fill(node.TrailingCommaFodder, false, false)
		if node.TrailingComma {
			u.Write(",")
		}
		u.unparseSpecs(&node.Spec)
		u.Fill(node.CloseFodder, true, u.options.PadArrays)
		u.Write("]")

	case *ast.Assert:
		u.Write("assert")
		u.Unparse(node.Cond, true)
		if node.Message != nil {
			u.Fill(node.ColonFodder, true, true)
			u.Write(":")
			u.Unparse(node.Message, true)
		}
		u.Fill(node.SemicolonFodder, false, false)
		u.Write(";")
		u.Unparse(node.Rest, true)

	case *ast.Binary:
		u.Unparse(node.Left, crowded)
		u.Fill(node.OpFodder, true, true)
		u.Write(node.Op.String())
		u.Unparse(node.Right, true)

	case *ast.Conditional:
		u.Write("if")
		u.Unparse(node.Cond, true)
		u.Fill(node.ThenFodder, true, true)
		u.Write("then")
		u.Unparse(node.BranchTrue, true)
		if node.BranchFalse != nil {
			u.Fill(node.ElseFodder, true, true)
			u.Write("else")
			u.Unparse(node.BranchFalse, true)
		}

	case *ast.Dollar:
		u.Write("$")

	case *ast.Error:
		u.Write("error")
		u.Unparse(node.Expr, true)

	case *ast.Function:
		u.Write("function")
		u.unparseParams(node.ParenLeftFodder, node.Parameters, node.TrailingComma, node.ParenRightFodder)
		u.Unparse(node.Body, true)

	case *ast.Import:
		u.Write("import")
		u.Unparse(node.File, true)

	case *ast.ImportStr:
		u.Write("importstr")
		u.Unparse(node.File, true)

	case *ast.Index:
		u.Unparse(node.Target, crowded)
		u.Fill(node.LeftBracketFodder, false, false) // Can also be DotFodder
		if node.Id != nil {
			u.Write(".")
			u.Fill(node.RightBracketFodder, false, false) // IdFodder
			u.unparseID(*node.Id)
		} else {
			u.Write("[")
			u.Unparse(node.Index, false)
			u.Fill(node.RightBracketFodder, false, false)
			u.Write("]")
		}

	case *ast.Slice:
		u.Unparse(node.Target, crowded)
		u.Fill(node.LeftBracketFodder, false, false)
		u.Write("[")
		if node.BeginIndex != nil {
			u.Unparse(node.BeginIndex, false)
		}
		u.Fill(node.EndColonFodder, false, false)
		u.Write(":")
		if node.EndIndex != nil {
			u.Unparse(node.EndIndex, false)
		}
		if node.Step != nil || len(node.StepColonFodder) > 0 {
			u.Fill(node.StepColonFodder, false, false)
			u.Write(":")
			if node.Step != nil {
				u.Unparse(node.Step, false)
			}
		}
		u.Fill(node.RightBracketFodder, false, false)
		u.Write("]")

	case *ast.InSuper:
		u.Unparse(node.Index, true)
		u.Fill(node.InFodder, true, true)
		u.Write("in")
		u.Fill(node.SuperFodder, true, true)
		u.Write("super")

	case *ast.Local:
		u.Write("local")
		if len(node.Binds) == 0 {
			panic("INTERNAL ERROR: local with no binds")
		}
		first := true
		for _, bind := range node.Binds {
			if !first {
				u.Write(",")
			}
			first = false
			u.Fill(bind.VarFodder, true, true)
			u.unparseID(bind.Variable)
			if bind.Fun != nil {
				u.unparseParams(bind.Fun.ParenLeftFodder,
					bind.Fun.Parameters,
					bind.Fun.TrailingComma,
					bind.Fun.ParenRightFodder)
			}
			u.Fill(bind.EqFodder, true, true)
			u.Write("=")
			u.Unparse(bind.Body, true)
			u.Fill(bind.CloseFodder, false, false)
		}
		u.Write(";")
		u.Unparse(node.Body, true)

	case *ast.LiteralBoolean:
		if node.Value {
			u.Write("true")
		} else {
			u.Write("false")
		}

	case *ast.LiteralNumber:
		u.Write(node.OriginalString)

	case *ast.LiteralString:
		switch node.Kind {
		case ast.StringDouble:
			u.Write("\"")
			// The original escape codes are still in the string.
			u.Write(node.Value)
			u.Write("\"")
		case ast.StringSingle:
			u.Write("'")
			// The original escape codes are still in the string.
			u.Write(node.Value)
			u.Write("'")
		case ast.StringBlock:
			u.Write("|||\n")
			if node.Value[0] != '\n' {
				u.Write(node.BlockIndent)
			}
			for i, r := range node.Value {
				// Formatter always outputs in unix mode.
				if r == '\r' {
					continue
				}
				u.Write(string(r))
				if r == '\n' && (i+1 < len(node.Value)) && node.Value[i+1] != '\n' {
					u.Write(node.BlockIndent)
				}
			}
			u.Write(node.BlockTermIndent)
			u.Write("|||")
		case ast.VerbatimStringDouble:
			u.Write("@\"")
			// Escapes were processed by the parser, so put them back in.
			for _, r := range node.Value {
				if r == '"' {
					u.Write("\"\"")
				} else {
					u.Write(string(r))
				}
			}
			u.Write("\"")
		case ast.VerbatimStringSingle:
			u.Write("@'")
			// Escapes were processed by the parser, so put them back in.
			for _, r := range node.Value {
				if r == '\'' {
					u.Write("''")
				} else {
					u.Write(string(r))
				}
			}
			u.Write("'")
		}

	case *ast.LiteralNull:
		u.Write("null")

	case *ast.Object:
		u.Write("{")
		u.unparseFields(node.Fields, u.options.PadObjects)
		if node.TrailingComma {
			u.Write(",")
		}
		u.Fill(node.CloseFodder, len(node.Fields) > 0, u.options.PadObjects)
		u.Write("}")

	case *ast.ObjectComp:
		u.Write("{")
		u.unparseFields(node.Fields, u.options.PadObjects)
		if node.TrailingComma {
			u.Write(",")
		}
		u.unparseSpecs(&node.Spec)
		u.Fill(node.CloseFodder, true, u.options.PadObjects)
		u.Write("}")

	case *ast.Parens:
		u.Write("(")
		u.Unparse(node.Inner, false)
		u.Fill(node.CloseFodder, false, false)
		u.Write(")")

	case *ast.Self:
		u.Write("self")

	case *ast.SuperIndex:
		u.Write("super")
		u.Fill(node.DotFodder, false, false)
		if node.Id != nil {
			u.Write(".")
			u.Fill(node.IDFodder, false, false)
			u.unparseID(*node.Id)
		} else {
			u.Write("[")
			u.Unparse(node.Index, false)
			u.Fill(node.IDFodder, false, false)
			u.Write("]")
		}
	case *ast.Var:
		u.unparseID(node.Id)

	case *ast.Unary:
		u.Write(node.Op.String())
		u.Unparse(node.Expr, false)

	default:
		panic(fmt.Sprintf("INTERNAL ERROR: Unknown AST: %T", expr))
	}
}

func (u *Unparser) String() string {
	return u.buf.String()
}
