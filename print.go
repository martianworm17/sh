// Copyright (c) 2016, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package sh

import (
	"fmt"
	"io"
)

func (p *printer) nodeJoin(ns []Node, sep string) {
	for i, n := range ns {
		if i > 0 {
			p.pr(sep)
		}
		p.node(n)
	}
}

func (p *printer) wordJoin(ws []Word, sep string) {
	for i, w := range ws {
		if i > 0 {
			p.pr(sep)
		}
		p.node(w)
	}
}

func (p *printer) stmtJoinWithEnd(stmts []Stmt, end bool) {
	p.newline = false
	for i, s := range stmts {
		if p.newline {
			p.newline = false
			p.pr("\n")
		} else if i > 0 {
			p.pr("; ")
		}
		p.node(s)
		p.newline = s.newlineAfter()
	}
}

func (p *printer) stmtJoin(stmts []Stmt) { p.stmtJoinWithEnd(stmts, true) }

func (p *printer) stmtList(stmts []Stmt) {
	if len(stmts) == 0 {
		p.pr(SEMICOLON, " ")
		return
	}
	p.pr(" ")
	p.stmtJoin(stmts)
	if p.newline {
		p.pr("\n")
	} else {
		p.pr(SEMICOLON, " ")
	}
}

func (p *printer) semicolonIfNil(v interface{}) {
	if v == nil {
		p.pr(SEMICOLON, " ")
		return
	}
	p.node(v)
}

func Fprint(w io.Writer, v interface{}) error {
	p := printer{w: w}
	p.node(v)
	return p.err
}

type printer struct {
	w   io.Writer
	err error

	newline bool
}

func (p *printer) pr(a ...interface{}) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprint(p.w, a...)
}

func (p *printer) node(v interface{}) {
	switch x := v.(type) {
	case File:
		p.stmtJoinWithEnd(x.Stmts, false)
	case Stmt:
		first := true
		if x.Negated {
			p.pr(NOT)
			first = false
		}
		for _, a := range x.Assigns {
			if !first {
				p.pr(" ")
			}
			p.node(a)
			first = false
		}
		if x.Node != nil {
			if !first {
				p.pr(" ")
			}
			p.node(x.Node)
			first = false
		}
		for _, r := range x.Redirs {
			if !first {
				p.pr(" ")
			}
			p.node(r.N)
			p.pr(r.Op)
			if _, ok := r.Word.Parts[0].(CmdInput); ok {
				p.pr(" ")
			}
			p.node(r.Word)
			first = false
		}
		if x.Background {
			if !first {
				p.pr(" ")
			}
			p.pr(AND)
		}
	case Assign:
		if x.Name != nil {
			p.node(x.Name)
			if x.Append {
				p.pr("+=")
			} else {
				p.pr("=")
			}
		}
		p.node(x.Value)
	case Command:
		for i, w := range x.Args {
			if i > 0 {
				p.pr(" ")
			}
			p.node(w)
		}
	case Subshell:
		p.pr(LPAREN)
		if len(x.Stmts) == 0 {
			// A space in between to avoid confusion with ()
			p.pr(" ")
		}
		p.stmtJoinWithEnd(x.Stmts, false)
		p.pr(RPAREN)
	case Block:
		p.pr(LBRACE)
		p.stmtList(x.Stmts)
		p.pr(RBRACE)
	case IfStmt:
		p.pr(IF)
		p.semicolonIfNil(x.Cond)
		p.pr(THEN)
		p.stmtList(x.ThenStmts)
		for _, el := range x.Elifs {
			p.pr(ELIF)
			p.semicolonIfNil(el.Cond)
			p.pr(THEN)
			p.stmtList(el.ThenStmts)
		}
		if len(x.ElseStmts) > 0 {
			p.pr(ELSE)
			p.stmtList(x.ElseStmts)
		}
		p.pr(FI)
	case StmtCond:
		p.stmtList(x.Stmts)
	case CStyleCond:
		p.pr(" ((")
		p.node(x.Cond)
		p.pr(")); ")
	case WhileStmt:
		p.pr(WHILE)
		p.semicolonIfNil(x.Cond)
		p.pr(DO)
		p.stmtList(x.DoStmts)
		p.pr(DONE)
	case UntilStmt:
		p.pr(UNTIL)
		p.semicolonIfNil(x.Cond)
		p.pr(DO)
		p.stmtList(x.DoStmts)
		p.pr(DONE)
	case ForStmt:
		p.pr(FOR, " ")
		p.node(x.Cond)
		p.pr("; ", DO)
		p.stmtList(x.DoStmts)
		p.pr(DONE)
	case WordIter:
		p.node(x.Name)
		if len(x.List) > 0 {
			p.pr(" ", IN, " ")
			p.wordJoin(x.List, " ")
		}
	case CStyleLoop:
		p.pr("((")
		p.node(x.Init)
		p.pr("; ")
		p.node(x.Cond)
		p.pr("; ")
		p.node(x.Post)
		p.pr("))")
	case UnaryExpr:
		if !x.Post {
			p.pr(x.Op)
		}
		p.node(x.X)
		if x.Post {
			p.pr(x.Op)
		}
	case BinaryExpr:
		p.node(x.X)
		if x.Op != COMMA {
			p.pr(" ")
		}
		p.pr(x.Op, " ")
		p.node(x.Y)
	case FuncDecl:
		if x.BashStyle {
			p.pr(FUNCTION, " ")
		}
		p.node(x.Name)
		p.pr("() ")
		p.node(x.Body)
	case Word:
		p.nodeJoin(x.Parts, "")
	case Lit:
		p.pr(x.Value)
	case SglQuoted:
		p.pr(SQUOTE, x.Value, SQUOTE)
	case Quoted:
		stop := x.Quote
		if stop == DOLLSQ {
			stop = SQUOTE
		} else if stop == DOLLDQ {
			stop = DQUOTE
		}
		p.pr(x.Quote)
		p.nodeJoin(x.Parts, "")
		p.pr(stop)
	case CmdSubst:
		if x.Backquotes {
			p.pr(BQUOTE)
		} else {
			p.pr(DOLLAR, "", LPAREN)
		}
		p.stmtJoin(x.Stmts)
		if x.Backquotes {
			p.pr(BQUOTE)
		} else {
			p.pr(RPAREN)
		}
	case ParamExp:
		if x.Short {
			p.pr(DOLLAR)
			p.node(x.Param)
			return
		}
		p.pr("${")
		if x.Length {
			p.pr(HASH)
		}
		p.node(x.Param)
		if x.Ind != nil {
			p.node(*x.Ind)
		}
		if x.Repl != nil {
			p.node(*x.Repl)
		}
		if x.Exp != nil {
			p.node(*x.Exp)
		}
		p.pr("}")
	case Index:
		p.pr(LBRACK)
		p.node(x.Word)
		p.pr(RBRACK)
	case Replace:
		if x.All {
			p.pr(QUO)
		}
		p.pr(QUO)
		p.node(x.Orig)
		p.pr(QUO)
		p.node(x.With)
	case Expansion:
		p.pr(x.Op)
		p.node(x.Word)
	case ArithmExpr:
		p.pr("$((")
		if x.X != nil {
			p.node(x.X)
		}
		p.pr("))")
	case ParenExpr:
		p.pr("(")
		p.node(x.X)
		p.pr(")")
	case CaseStmt:
		p.pr(CASE, " ")
		p.node(x.Word)
		p.pr(" ", IN)
		for i, pl := range x.List {
			if i > 0 {
				p.pr(";;")
			}
			p.pr(" ")
			p.wordJoin(pl.Patterns, " | ")
			p.pr(") ")
			p.stmtJoin(pl.Stmts)
		}
		p.pr("; ", ESAC)
	case DeclStmt:
		if x.Local {
			p.pr(LOCAL)
		} else {
			p.pr(DECLARE)
		}
		for _, w := range x.Opts {
			p.pr(" ")
			p.node(w)
		}
		for _, a := range x.Assigns {
			p.pr(" ")
			p.node(a)
		}
	case ArrayExpr:
		p.pr(LPAREN)
		p.wordJoin(x.List, " ")
		p.pr(RPAREN)
	case CmdInput:
		p.pr(CMDIN)
		p.stmtJoin(x.Stmts)
		p.pr(RPAREN)
	case EvalStmt:
		p.pr(EVAL, " ")
		p.node(x.Stmt)
	case LetStmt:
		p.pr(LET, " ")
		p.nodeJoin(x.Exprs, " ")
	}
}