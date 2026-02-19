package core

import "strings"

func rewriteExpression(expr string) (string, map[string]string) {
	var out strings.Builder
	out.Grow(len(expr))

	varMap := make(map[string]string)
	inString := false
	var stringQuote byte

	for i := 0; i < len(expr); {
		ch := expr[i]

		if inString {
			out.WriteByte(ch)
			if ch == '\\' && i+1 < len(expr) {
				out.WriteByte(expr[i+1])
				i += 2
				continue
			}
			if ch == stringQuote {
				inString = false
			}
			i++
			continue
		}

		if ch == '"' || ch == '\'' {
			inString = true
			stringQuote = ch
			out.WriteByte(ch)
			i++
			continue
		}

		if ch == '$' || isIdentStart(ch) {
			if isIdentStart(ch) {
				if ident, next := parseIdentifier(expr, i); isFunctionName(ident) && nextNonSpaceIs(expr, next, '(') {
					out.WriteString(ident)
					i = next
					continue
				}
			}
			token, next := parseJSONPathToken(expr, i)
			if isKeyword(token) {
				out.WriteString(token)
				i = next
				continue
			}
			path := token
			if token[0] != '$' {
				path = "$." + token
			}
			safe := safeVarName(path)
			varMap[safe] = path
			out.WriteString(safe)
			i = next
			continue
		}

		out.WriteByte(ch)
		i++
	}

	return out.String(), varMap
}

func isFunctionName(token string) bool {
	switch token {
	case "contains", "like":
		return true
	default:
		return false
	}
}

func parseIdentifier(expr string, start int) (string, int) {
	i := start
	for i < len(expr) {
		ch := expr[i]
		if isIdentStart(ch) || (ch >= '0' && ch <= '9') {
			i++
			continue
		}
		break
	}
	return expr[start:i], i
}

func nextNonSpaceIs(expr string, start int, want byte) bool {
	for i := start; i < len(expr); i++ {
		switch expr[i] {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return expr[i] == want
		}
	}
	return false
}

func parseJSONPathToken(expr string, start int) (string, int) {
	i := start
	bracketDepth := 0
	parenDepth := 0
	var quote byte

	for i < len(expr) {
		ch := expr[i]

		if quote != 0 {
			if ch == '\\' && i+1 < len(expr) {
				i += 2
				continue
			}
			if ch == quote {
				quote = 0
			}
			i++
			continue
		}

		switch ch {
		case '\'', '"':
			quote = ch
			i++
			continue
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '(':
			if bracketDepth > 0 {
				parenDepth++
			}
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		}

		if bracketDepth == 0 && parenDepth == 0 && isTerminator(ch) {
			break
		}

		i++
	}
	return expr[start:i], i
}

func isTerminator(ch byte) bool {
	switch ch {
	case ' ', '\t', '\n', '\r', ',', ';':
		return true
	case '+', '-', '*', '/', '%':
		return true
	case '=', '!', '<', '>', '&', '|':
		return true
	case ')':
		return true
	default:
		return false
	}
}

func safeVarName(token string) string {
	var b strings.Builder
	b.Grow(len(token) + 2)
	b.WriteString("v_")
	for i := 0; i < len(token); i++ {
		ch := token[i]
		if isIdentStart(ch) || (ch >= '0' && ch <= '9') {
			b.WriteByte(ch)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isKeyword(token string) bool {
	switch token {
	case "true", "false", "null":
		return true
	default:
		return false
	}
}
