// Package reportdoc produces portable Word reports without an office runtime.
package reportdoc

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const MIME = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

type Document struct {
	body     strings.Builder
	Portrait bool
}

func escape(value string) string {
	value = strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\t' && r != '\r' {
			return -1
		}
		return r
	}, value)
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(value))
	return b.String()
}

func paragraph(text, style string) string {
	var b strings.Builder
	b.WriteString(`<w:p><w:pPr><w:pStyle w:val="` + style + `"/></w:pPr><w:r>`)
	for i, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if i > 0 {
			b.WriteString(`<w:br/>`)
		}
		b.WriteString(`<w:t xml:space="preserve">` + escape(line) + `</w:t>`)
	}
	b.WriteString(`</w:r></w:p>`)
	return b.String()
}

func (d *Document) Title(text string)   { d.body.WriteString(paragraph(text, "Title")) }
func (d *Document) Heading(text string) { d.body.WriteString(paragraph(text, "Heading1")) }
func (d *Document) Text(text string)    { d.body.WriteString(paragraph(text, "Normal")) }
func (d *Document) Lead(text string)    { d.body.WriteString(paragraph(text, "Lead")) }

func (d *Document) Table(headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}
	total, first := 14400, 2600
	if d.Portrait {
		total, first = 9300, 1800
	}
	widths := make([]int, len(headers))
	for i := range widths {
		widths[i] = total / len(headers)
	}
	if len(headers) > 1 {
		widths[0] = first
		for i := 1; i < len(headers); i++ {
			widths[i] = (total - first) / (len(headers) - 1)
		}
	}
	fmt.Fprintf(&d.body, `<w:tbl><w:tblPr><w:tblW w:w="%d" w:type="dxa"/><w:tblLayout w:type="fixed"/><w:tblBorders>`, total)
	for _, edge := range []string{"top", "left", "bottom", "right", "insideH", "insideV"} {
		fmt.Fprintf(&d.body, `<w:%s w:val="single" w:sz="4" w:color="D9D9D9"/>`, edge)
	}
	d.body.WriteString(`</w:tblBorders><w:tblCellMar><w:top w:w="100" w:type="dxa"/><w:left w:w="120" w:type="dxa"/><w:bottom w:w="100" w:type="dxa"/><w:right w:w="120" w:type="dxa"/></w:tblCellMar></w:tblPr><w:tblGrid>`)
	for _, width := range widths {
		fmt.Fprintf(&d.body, `<w:gridCol w:w="%d"/>`, width)
	}
	d.body.WriteString(`</w:tblGrid>`)
	all := append([][]string{headers}, rows...)
	for index, row := range all {
		d.body.WriteString(`<w:tr><w:trPr>`)
		if index == 0 {
			d.body.WriteString(`<w:tblHeader/>`)
		}
		d.body.WriteString(`</w:trPr>`)
		for column := range headers {
			fill, style := "FFFFFF", "TableText"
			if index == 0 {
				fill, style = "E8EFF5", "TableHeader"
			} else if index%2 == 0 {
				fill = "F7F9FB"
			}
			fmt.Fprintf(&d.body, `<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/><w:shd w:fill="%s"/><w:vAlign w:val="center"/></w:tcPr>`, widths[column], fill)
			value := ""
			if column < len(row) {
				value = row[column]
			}
			d.body.WriteString(paragraph(value, style))
			d.body.WriteString(`</w:tc>`)
		}
		d.body.WriteString(`</w:tr>`)
	}
	d.body.WriteString(`</w:tbl>`)
	// Word needs a paragraph after a table; a normal-height empty paragraph can
	// otherwise create a completely blank trailing page.
	d.body.WriteString(`<w:p><w:pPr><w:spacing w:before="0" w:after="0" w:line="20" w:lineRule="exact"/></w:pPr><w:r><w:rPr><w:sz w:val="2"/></w:rPr><w:t></w:t></w:r></w:p>`)
}

func (d *Document) Bytes() ([]byte, error) {
	files := []struct{ name, text string }{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`},
		{"word/_rels/document.xml.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`},
		{"word/styles.xml", `<?xml version="1.0" encoding="UTF-8"?><w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:eastAsia="Microsoft YaHei"/><w:sz w:val="21"/><w:lang w:val="zh-CN"/></w:rPr></w:rPrDefault><w:pPrDefault><w:pPr><w:spacing w:after="120" w:line="280" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults><w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style><w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:after="220"/><w:keepNext/></w:pPr><w:rPr><w:b/><w:color w:val="000000"/><w:sz w:val="36"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:pPr><w:keepNext/><w:spacing w:before="240" w:after="120"/><w:outlineLvl w:val="0"/></w:pPr><w:rPr><w:b/><w:sz w:val="27"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="TableText"><w:name w:val="Table Text"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:after="40" w:line="260" w:lineRule="auto"/></w:pPr><w:rPr><w:sz w:val="20"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="TableHeader"><w:name w:val="Table Header"/><w:basedOn w:val="TableText"/><w:rPr><w:b/><w:color w:val="000000"/></w:rPr></w:style></w:styles>`},
		{"word/document.xml", `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + d.body.String() + `<w:sectPr><w:pgSz w:w="16838" w:h="11906" w:orient="landscape"/><w:pgMar w:top="1000" w:right="1219" w:bottom="1000" w:left="1219" w:header="500" w:footer="500"/></w:sectPr></w:body></w:document>`},
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, file := range files {
		if file.name == "word/styles.xml" {
			file.text = strings.Replace(file.text, `</w:styles>`, `<w:style w:type="paragraph" w:styleId="Lead"><w:name w:val="Summary Lead"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:after="180"/><w:keepNext/></w:pPr><w:rPr><w:b/><w:color w:val="000000"/><w:sz w:val="24"/></w:rPr></w:style></w:styles>`, 1)
		}
		if d.Portrait && file.name == "word/document.xml" {
			file.text = strings.Replace(file.text, `<w:pgSz w:w="16838" w:h="11906" w:orient="landscape"/>`, `<w:pgSz w:w="11906" w:h="16838"/>`, 1)
		}
		entry, err := writer.Create(file.name)
		if err != nil {
			return nil, err
		}
		if _, err = io.WriteString(entry, file.text); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func Download(w http.ResponseWriter, filename string, data []byte) {
	w.Header().Set("Content-Type", MIME)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.docx"`, filename))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
