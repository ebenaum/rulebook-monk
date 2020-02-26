package rulebook

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type itemType uint8

const (
	doubleNewLine = "\n\n"
	newLine       = "\n"
	chapter       = "#"
	boldRune      = '*'
	emSymbol      = "__"
	section       = "##"
	table         = "-table-"
	annex         = "ANNEX"
	listElement   = "\n- "
	link          = "["
	cmdStart      = '\\'
	eof           = 0
)

const (
	itemError itemType = iota

	itemText
	itemBold
	itemEm
	itemNewLine
	itemChapter
	itemSection
	itemAnnex
	itemStartListElement
	itemEndListElement
	itemListOpen
	itemLink
	itemCommand
	itemListClose
	itemTableStart
	itemTableEnd
	itemTableRow
	itemEOF
)

type item struct {
	typ  itemType
	val  string
	line int
}

type stateFn func(*lexer) stateFn

type lexer struct {
	input string // the string being scanned.
	start int    // start position of this item.
	pos   int    // current position in the input.
	width int    // width of last rune read from input.
	line  int
	items chan item // channel of scanned items.
	state stateFn
}

func (itype itemType) String() string {
	switch itype {
	case itemError:
		return "Error"
	case itemEOF:
		return "EOF"
	case itemText:
		return "Text"
	case itemEm:
		return "Em"
	case itemBold:
		return "Bold"
	case itemSection:
		return "Section"
	case itemChapter:
		return "Chapter"
	case itemAnnex:
		return "Annex"
	case itemLink:
		return "Link"
	case itemCommand:
		return "Command"
	case itemNewLine:
		return "NewLine"
	case itemStartListElement:
		return "StartListElement"
	case itemEndListElement:
		return "EndListElement"
	case itemListOpen:
		return "ListOpen"
	case itemListClose:
		return "ListClose"
	case itemTableStart:
		return "TableStart"
	case itemTableEnd:
		return "TableEnd"
	case itemTableRow:
		return "TableRow"
	}
	panic(fmt.Sprintf("BUG: Unknown type '%d'.", int(itype)))
}

func (item item) String() string {
	return fmt.Sprintf("%s: %s (line %v)", item.typ.String(), item.val, item.line)
}

func lex(input string) *lexer {
	l := &lexer{
		input: input,
		state: lexText,
		line:  1,
		items: make(chan item, 3),
	}

	return l
}

func (l *lexer) nextItem() item {
	for {
		select {
		case item := <-l.items:
			return item
		default:
			l.state = l.state(l)
		}
	}
}

func (l *lexer) emit(t itemType) {
	l.items <- item{t, l.input[l.start:l.pos], l.line}
	l.start = l.pos
}

func (l *lexer) emitCustom(t itemType, data string) {
	l.items <- item{t, data, l.line}
}

func (l *lexer) emitTrim(typ itemType) {
	l.items <- item{typ, strings.TrimSpace(l.input[l.start:l.pos]), l.line}
	l.start = l.pos
}

func (l *lexer) errorf(format string, values ...interface{}) stateFn {
	l.items <- item{
		itemError,
		fmt.Sprintf(format, values...),
		l.line,
	}

	return nil
}

func (l *lexer) ignore() {
	l.start = l.pos
}

func (l *lexer) backup() {
	l.pos -= l.width
	if l.pos < len(l.input) && l.input[l.pos] == '\n' {
		l.line--
	}
}

func (l *lexer) next() (rune rune) {
	if l.pos >= len(l.input) {
		l.width = 0
		return eof
	}
	rune, l.width = utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += l.width

	if rune == '\n' {
		l.line++
	}

	return rune
}

func (l *lexer) peek() rune {
	rune := l.next()
	l.backup()
	return rune
}

func lexText(l *lexer) stateFn {
	for {
		if strings.HasPrefix(l.input[l.pos:], section) {
			if l.pos > l.start {
				l.emit(itemText)
			}

			l.next()
			l.next()
			l.ignore()
			return lexSection
		}

		if strings.HasPrefix(l.input[l.pos:], table) {
			if l.pos > l.start {
				l.emit(itemText)
			}

			return lexTableTitle(lexText)
		}

		if strings.HasPrefix(l.input[l.pos:], annex) {
			if l.pos > l.start {
				l.emit(itemText)
			}

			l.pos += 5
			l.ignore()
			return lexAnnex
		}

		if strings.HasPrefix(l.input[l.pos:], chapter) {
			if l.pos > l.start {
				l.emit(itemText)
			}

			l.next()
			l.ignore()
			return lexChapter
		}

		if strings.HasPrefix(l.input[l.pos:], listElement) {
			if l.pos > l.start {
				l.emit(itemText)
			}
			l.next()
			l.next()
			l.next()
			l.ignore()
			l.emitTrim(itemListOpen)
			l.emit(itemStartListElement)
			return lexListItem
		}

		if strings.HasPrefix(l.input[l.pos:], newLine) {
			if l.pos > l.start {
				l.emit(itemText)
			}

			l.emitTrim(itemNewLine)

			l.next()
			l.ignore()

			return lexText
		}

		if strings.HasPrefix(l.input[l.pos:], emSymbol) {
			if l.pos > l.start {
				l.emit(itemText)
			}

			l.next()
			l.next()

			return lexEm(lexText)
		}

		next := l.next()
		if next == cmdStart {
			if l.pos > l.start {
				l.backup()
				l.emit(itemText)
				l.next()
			}

			return lexCmdName(lexText)
		}

		if next == boldRune {
			if l.pos > l.start {
				l.backup()
				l.emit(itemText)
				l.next()
			}

			return lexBold(lexText)
		}

		if next == '[' {
			if l.pos > l.start {
				l.backup()
				l.emit(itemText)
				l.next()
			}

			return lexLinkHead(lexText)
		}

		if next == eof {
			break
		}
	}

	if l.pos > l.start {
		l.emitTrim(itemText)
	}

	l.emit(itemEOF)
	return nil
}

func lexChapter(l *lexer) stateFn {
	for {
		if strings.HasPrefix(l.input[l.pos:], newLine) {
			l.emitTrim(itemChapter)
			return lexText
		}

		if l.next() == eof {
			l.backup()
			return lexText
		}

	}
}

func lexSection(l *lexer) stateFn {
	for {
		if strings.HasPrefix(l.input[l.pos:], newLine) {
			l.emitTrim(itemSection)
			return lexText
		}

		if l.next() == eof {
			l.backup()
			return lexText
		}

	}
}

func lexAnnex(l *lexer) stateFn {
	for {
		if strings.HasPrefix(l.input[l.pos:], newLine) {
			l.emitTrim(itemAnnex)
			return lexText
		}

		if l.next() == eof {
			l.backup()
			return lexText
		}

	}
}

func lexListItem(l *lexer) stateFn {
	for {

		if strings.HasPrefix(l.input[l.pos:], listElement) {
			if l.pos > l.start {
				l.emit(itemText)
			}
			l.next()
			l.next()
			l.next()
			l.ignore()
			l.emit(itemEndListElement)
			l.emit(itemStartListElement)
			return lexListItem
		}

		if strings.HasPrefix(l.input[l.pos:], newLine) {
			if l.pos > l.start {
				l.emit(itemText)
			}
			l.emitTrim(itemEndListElement)
			l.emitTrim(itemListClose)
			return lexText
		}

		if strings.HasPrefix(l.input[l.pos:], emSymbol) {
			if l.pos > l.start {
				l.emit(itemText)
			}

			l.next()
			l.next()

			return lexEm(lexListItem)
		}

		next := l.next()
		if next == boldRune {
			if l.pos > l.start {
				l.backup()
				l.emit(itemText)
				l.next()
			}

			return lexBold(lexListItem)
		}

		if next == eof {
			l.backup()
			return lexText
		}

	}
}

func lexBold(fn stateFn) stateFn {
	return func(l *lexer) stateFn {
		l.ignore()
		for {
			next := l.next()

			if next == boldRune {
				l.backup()
				l.emit(itemBold)
				l.next()
				l.ignore()
				return fn
			}

		}
	}
}

func lexEm(fn stateFn) stateFn {
	return func(l *lexer) stateFn {
		l.ignore()
		for {

			if strings.HasPrefix(l.input[l.pos:], emSymbol) {
				l.emit(itemEm)
				l.next()
				l.next()
				l.ignore()
				return fn
			}

			l.next()
		}
	}
}

func lexTableTitle(fn stateFn) stateFn {
	return func(l *lexer) stateFn {
		l.pos += len(table)
		l.ignore()

		for {
			next := l.next()
			if next == rune(newLine[0]) {
				l.emitTrim(itemTableStart)
				return lexTable(fn)
			}
		}
	}
}

func lexTable(fn stateFn) stateFn {
	return func(l *lexer) stateFn {
		for {

			if strings.HasPrefix(l.input[l.pos:], table) {
				l.emit(itemTableEnd)
				l.pos += len(table)
				l.ignore()
				return fn
			}

			next := l.next()
			if next == rune(newLine[0]) {
				l.emitTrim(itemTableRow)
				return lexTable(fn)
			}

		}
	}
}

func lexCmdName(fn stateFn) stateFn {
	return func(l *lexer) stateFn {
		l.ignore()
		for {

			next := l.next()
			if next == '(' {
				cmd := l.input[l.start : l.pos-1]
				l.ignore()
				return lexCmdArgs(cmd, fn)
			}

		}
	}
}

func lexCmdArgs(cmd string, fn stateFn) stateFn {
	return func(l *lexer) stateFn {
		for {
			next := l.next()
			if next == ')' {
				l.backup()
				l.emitCustom(itemCommand, fmt.Sprintf("%s|%s", cmd, l.input[l.start:l.pos]))
				l.next()
				l.ignore()
				return fn
			}

		}
	}
}

func lexLinkHead(fn stateFn) stateFn {
	return func(l *lexer) stateFn {
		l.ignore()
		for {

			next := l.next()
			if next == ']' {
				text := l.input[l.start : l.pos-1]
				l.next()
				l.ignore()
				return lexLinkTail(text, fn)
			}

		}
	}
}

func lexLinkTail(text string, fn stateFn) stateFn {
	return func(l *lexer) stateFn {
		for {
			next := l.next()
			if next == ')' {
				l.backup()
				l.emitCustom(itemLink, fmt.Sprintf("%s|%s", text, l.input[l.start:l.pos]))
				l.next()
				l.ignore()
				return fn
			}

		}
	}
}
