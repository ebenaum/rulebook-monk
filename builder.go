package rulebook

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"
)

type Section struct {
	Title string
	Items []item
}

type Chapter struct {
	Title    string
	Items    []item
	Sections []Section
}

type Document struct {
	Items    []item
	Sections []Section
	Chapters []Chapter
	Annexes  []Section
}

func toAnnex(n int) string {
	return string(n + 65)
}

func Build(input io.Reader, w io.Writer, config BuilderConfig) error {
	b, err := ioutil.ReadAll(input)
	if err != nil {
		return err
	}

	lexer := lex(strings.Replace(string(b), "%", "%%", -1))

	document := Document{Chapters: make([]Chapter, 0), Items: make([]item, 0), Sections: make([]Section, 0)}

	var chapter *Chapter
	var sections *[]Section
	var items *[]item
	var section *Section

	sections = &document.Sections
	items = &document.Items

	var it item
	for it = lexer.nextItem(); it.typ != itemEOF && it.typ != itemError; it = lexer.nextItem() {
		switch it.typ {
		case itemChapter:
			document.Chapters = append(document.Chapters, Chapter{Items: []item{}, Sections: make([]Section, 0)})
			chapter = &document.Chapters[len(document.Chapters)-1]
			chapter.Title = it.val
			sections = &chapter.Sections
			items = &chapter.Items
			section = nil
		case itemAnnex:
			document.Annexes = append(document.Annexes, Section{Items: []item{}, Title: it.val})
			annex := &document.Annexes[len(document.Annexes)-1]
			chapter = nil
			items = &annex.Items
		case itemSection:
			*sections = append(*sections, Section{Items: []item{}})
			section = &((*sections)[len(*sections)-1])
			section.Title = it.val
			items = &section.Items
		default:
			*items = append(*items, it)
		}
	}

	if it.typ == itemError {
		return fmt.Errorf("lexer error %+v", it)
	}

	builder := Builder{Config: config}

	out, err := builder.Build(document)
	if err != nil {
		return err
	}

	_, err = w.Write([]byte(out))

	return err
}

type BuilderConfig struct {
	TableOfContents bool
}

type Builder struct {
	err             error
	content         strings.Builder
	paragraphIsOpen bool
	newSection      bool
	tableRowIndex   int
	tableTitle      string

	Config BuilderConfig
}

func (b *Builder) closeParagraph() {
	if b.paragraphIsOpen {
		b.paragraphIsOpen = false
		b.append("\n</p>\n")
	}
}

func (b *Builder) openParagraph() {
	if !b.paragraphIsOpen && b.newSection {
		b.paragraphIsOpen = true
		b.newSection = false
		b.append("<p class='indent'>\n")
	} else if !b.paragraphIsOpen {
		b.paragraphIsOpen = true
		b.append("<p>\n")
	}
}

func (b *Builder) append(s string, args ...interface{}) {
	if b.err != nil {
		return
	}

	_, err := b.content.WriteString(fmt.Sprintf(s, args...))
	b.err = err
}

func anchorName(s string) string {
	s = strings.ToLower(s)
	s = strings.Replace(s, " ", "-", -1)

	return s
}

func annexAnchorName(s string) string {
	return fmt.Sprintf("annex-%s", anchorName(s))
}

func (b *Builder) handleItem(it item) {
	if it.typ == itemNewLine {
		b.closeParagraph()
	} else if it.typ == itemListOpen {
		b.closeParagraph()
		b.append("<ol class='roman'>\n")
	} else if it.typ == itemListClose {
		b.append("</ol>\n\n")
	} else if it.typ == itemStartListElement {
		b.append("\n<li>\n")
		b.openParagraph()
	} else if it.typ == itemEndListElement {
		b.closeParagraph()
		b.append("\n</li>\n")
	} else if it.typ == itemBold {
		b.openParagraph()
		b.append("<strong>%s</strong>", it.val)
	} else if it.typ == itemCommand {
		info := strings.Split(it.val, "|")
		b.handleCommand(info[0], strings.Split(info[1], ","))
	} else if it.typ == itemLink {
		b.openParagraph()
		info := strings.Split(it.val, "|")
		text, link := info[0], info[1]

		b.append("<a href='#%s'>%s</a>", anchorName(link), text)
	} else if it.typ == itemEm {
		b.openParagraph()
		b.append("<em>%s</em>", it.val)
	} else if it.typ == itemTableStart {
		b.closeParagraph()
		b.tableRowIndex = -1
		b.tableTitle = it.val
		b.append("<table>\n")
	} else if it.typ == itemTableRow {
		cells := strings.Split(it.val, "|")
		if b.tableRowIndex == -1 {
			b.append("<thead>\n")
			b.append("<tr>\n")
			b.append("<th colspan='%v'>%s</th>\n", len(cells), b.tableTitle)
			b.append("</tr>\n")
			b.append("</thead>\n")
			b.append("<tbody>\n")
		}
		b.tableRowIndex += 1
		b.append("<tr>\n")
		b.append("<td class='head'>%s</td>\n", cells[0])
		for _, cell := range cells[1:] {
			if b.tableRowIndex == 0 {
				b.append("<td class='head'>%s</td>\n", cell)
			} else {
				b.append("<td class='lead'>%s</td>\n", cell)
			}
		}
		b.append("</tr>\n")

	} else if it.typ == itemTableEnd {
		b.append("</tbody>\n")
		b.append("</table>\n")
	} else {
		if it.val != "" {
			b.openParagraph()
			b.append(it.val)
		}
	}
}

func (b *Builder) handleCommand(name string, args []string) {
	var classNames []string = []string{"illustration"}

	switch name {
	case "color":
		b.append("<span style='color: #%s'>%s</span>", strings.TrimSpace(args[1]), strings.TrimSpace(args[0]))
	case "img":
		b.closeParagraph()
		if len(args) > 2 {
			position := strings.TrimSpace(args[2])
			switch position {
			case "left":
				classNames = append(classNames, "float-left")
			case "right":
				classNames = append(classNames, "float-right")
			case "center":
			}
		}

		width := ""
		height := ""
		if len(args) > 3 {
			size := strings.TrimSpace(args[3])
			if size[0] == 'w' {
				width = size[1:]
			} else if size[0] == 'h' {
				height = size[1:]
			}
		}

		if width != "" {
			b.append("<img class='%s' src='%s' alt='%s' width='%s'/>", strings.Join(classNames, " "), args[0], strings.TrimSpace(args[1]), width)
		} else if height != "" {
			b.append("<img class='%s' src='%s' alt='%s' height='%s'/>", strings.Join(classNames, " "), args[0], strings.TrimSpace(args[1]), height)
		} else {
			b.append("<img class='%s' src='%s' alt='%s' />", strings.Join(classNames, " "), args[0], strings.TrimSpace(args[1]))
		}
	}

}

func (b *Builder) buildAnnex(annex Section) {
	b.newSection = true
	for _, it := range annex.Items {
		b.handleItem(it)
	}
}

func (b *Builder) handleSection(section Section) {
	b.newSection = true
	b.append("<h3><a name='%s'></a>%s</h3>\n", anchorName(section.Title), section.Title)
	for _, it := range section.Items {
		b.handleItem(it)
	}
}

func (b *Builder) buildTableOfContents(document Document) {
	b.append("<div id='summary'>\n<h3>Table des matières</h3>\n")
	b.append("<ol>\n")
	for _, section := range document.Sections {
		b.append("<li><a href='#%s'>%s</a></li>\n", anchorName(section.Title), section.Title)
	}
	b.append("</ol>\n")

	b.append("<ol>\n")
	for chapterIndex, chapter := range document.Chapters {
		b.append("<li><strong>%s</strong> - <a href='#%s'>%s</a></li>\n", toRoman(chapterIndex), anchorName(chapter.Title), chapter.Title)
		b.append("<ol class='roman'>\n")
		for _, section := range chapter.Sections {
			b.append("<li><a href='#%s'>%s</a></li>\n", anchorName(section.Title), section.Title)
		}
		b.append("</ol>\n")
	}
	b.append("</ol>\n")

	b.append("<ol>\n")
	for annexIndex, annex := range document.Annexes {
		b.append("<li><strong>Annexe %s</strong>: <a href='#%s'>%s</a></li>\n", toAnnex(annexIndex), annexAnchorName(annex.Title), annex.Title)
	}
	b.append("</ol>\n")

	b.append("</div>\n")
}

func (b *Builder) Build(document Document) (string, error) {
	b.content.Reset()
	b.paragraphIsOpen = false

	if b.Config.TableOfContents {
		b.buildTableOfContents(document)
	}

	for _, section := range document.Sections {
		b.handleSection(section)
	}

	for chapterIndex, chapter := range document.Chapters {
		b.newSection = true
		b.append("<h2><a id='%s'></a>%s - %s</h2>\n", anchorName(chapter.Title), toRoman(chapterIndex), chapter.Title)
		for _, it := range chapter.Items {
			b.handleItem(it)
		}

		for _, section := range chapter.Sections {
			b.handleSection(section)
		}
	}

	for annexIndex, annex := range document.Annexes {
		b.append("<div class='annex'>\n")
		b.append("<h2><a name='%s'></a>Annexe %v: %s</h2>\n", annexAnchorName(annex.Title), toAnnex(annexIndex), annex.Title)
		b.newSection = true
		for _, it := range annex.Items {
			b.handleItem(it)
		}
		b.append("</div>\n")
	}

	return b.content.String(), b.err
}
