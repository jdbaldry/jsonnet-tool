package main

import (
	"strings"

	"github.com/google/go-jsonnet/ast"
	"github.com/jdbaldry/jsonnet-tool/internal/parser"
)

// symbol is a referencable symbol in a Jsonnet file.
type symbol struct {
	Identifier    string
	Type          string
	Context       string
	LocationRange LocationRange
}

// findSymbols finds all the Jsonnet symbols that can be referenced by some variable or index.
// This includes object fields and local variables.
func findSymbols(node *ast.Node, context []string) (symbols []symbol, err error) {
	switch i := (*node).(type) {
	case *ast.DesugaredObject:
		for _, local := range i.Locals {
			symbols = append(symbols, symbol{
				Identifier: string(local.Variable),
				Type:       "objlocal",
				Context:    strings.Join(context, "."),
				LocationRange: LocationRange{
					FileName: i.Loc().FileName,
					Begin:    i.Loc().Begin,
					End:      i.Loc().End,
				}})
		}
		for _, field := range i.Fields {
			// TODO: evaluate expressions.
			switch name := field.Name.(type) {
			case *ast.LiteralString:
				symbols = append(symbols, symbol{
					Identifier: name.Value,
					Context:    strings.Join(context, "."),
					Type:       "field",
					LocationRange: LocationRange{
						FileName: i.Loc().FileName,
						Begin:    i.Loc().Begin,
						End:      i.Loc().End,
					}})
				children, err := findSymbols(&field.Body, append(context, name.Value))
				if err != nil {
					return symbols, err
				}
				symbols = append(symbols, children...)
			}
		}

	case *ast.Local:
		for _, bind := range i.Binds {
			symbols = append(symbols, symbol{
				Identifier: string(bind.Variable),
				Type:       "local",
				Context:    strings.Join(context, "."),
				LocationRange: LocationRange{
					FileName: bind.LocRange.FileName,
					Begin:    bind.LocRange.Begin,
					End:      bind.LocRange.End,
				}})
		}
		for _, node := range parser.Children(i) {
			additional, err := findSymbols(&node, context)
			if err != nil {
				return symbols, err
			}
			symbols = append(symbols, additional...)
		}

	default:
		for _, node := range parser.Children(i) {
			additional, err := findSymbols(&node, context)
			if err != nil {
				return symbols, err
			}
			symbols = append(symbols, additional...)
		}
	}
	return
}
