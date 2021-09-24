package main

import (
	"github.com/google/go-jsonnet/ast"
	"github.com/jdbaldry/jsonnet-tool/internal/parser"
)

// symbol is a referencable symbol in a Jsonnet file.
type symbol struct {
	Identifier    string
	Context       ast.Context
	LocationRange LocationRange
}

// findSymbols finds all the Jsonnet symbols that can be referenced by some variable or index.
// This includes object fields and local variables.
func findSymbols(node *ast.Node) (symbols []symbol, err error) {
	switch i := (*node).(type) {
	case *ast.DesugaredObject:
		for _, local := range i.Locals {
			symbols = append(symbols, symbol{
				Identifier: string(local.Variable),
				Context:    i.Context(),
				LocationRange: LocationRange{
					FileName: i.Loc().FileName,
					Begin:    i.Loc().Begin,
					End:      i.Loc().End,
				}})
		}
		// The direct children of a DesugaredObject node are the field keys.
		// TODO: evaluate expressions
		for _, node := range parser.DirectChildren(i) {
			switch j := node.(type) {
			case *ast.LiteralString:
				symbols = append(symbols, symbol{
					Identifier: j.Value,
					Context:    i.Context(),
					LocationRange: LocationRange{
						FileName: i.Loc().FileName,
						Begin:    i.Loc().Begin,
						End:      i.Loc().End,
					}})
			}
		}

		// The special children of a DesugaredObject node are the field values that are themselvs not symbols
		// but that may have symbols within them (in the case that the value is an object).
		for _, node := range parser.SpecialChildren(i) {
			additional, err := findSymbols(&node)
			if err != nil {
				return symbols, err
			}
			symbols = append(symbols, additional...)
		}

	case *ast.Local:
		for _, bind := range i.Binds {
			symbols = append(symbols, symbol{
				Identifier: string(bind.Variable),
				LocationRange: LocationRange{
					FileName: bind.LocRange.FileName,
					Begin:    bind.LocRange.Begin,
					End:      bind.LocRange.End,
				}})
		}
		for _, node := range parser.Children(i) {
			additional, err := findSymbols(&node)
			if err != nil {
				return symbols, err
			}
			symbols = append(symbols, additional...)
		}

	default:
		for _, node := range parser.Children(i) {
			additional, err := findSymbols(&node)
			if err != nil {
				return symbols, err
			}
			symbols = append(symbols, additional...)
		}
	}
	return
}
