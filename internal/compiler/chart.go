// Shared native chart XML (c:chartSpace) for XLSX + PPTX. Self-contained literals.
package compiler

import (
	"fmt"
	"strings"

	"github.com/you/office-brains/internal/oir"
)

// Truncated records content dropped during the last Build* call
// (slide overflow, missing images). CLI surfaces it as warnings so agents
// never mistake a truncated file for a complete one.
var Truncated []string

func warn(format string, a ...any) {
	Truncated = append(Truncated, fmt.Sprintf(format, a...))
}

func resetWarnings() { Truncated = nil }

var chartPalette = []string{"4472C4", "ED7D31", "A5A5A5", "FFC000", "5B9BD5", "70AD47", "264478", "E15728"}

// BarChartXML renders a clustered column chart with inline literals (no cell refs).
func BarChartXML(title string, labels []string, series []oir.ChartSeries) string {
	return ChartXML("bar", title, labels, series)
}

// ChartXML renders bar|line|pie charts with inline literals.
func ChartXML(kind, title string, labels []string, series []oir.ChartSeries) string {
	if kind == "" || kind == "bar" {
		return barLineChartXML(false, title, labels, series)
	}
	if kind == "line" {
		return barLineChartXML(true, title, labels, series)
	}
	return pieChartXML(title, labels, series)
}

func barLineChartXML(isLine bool, title string, labels []string, series []oir.ChartSeries) string {
	if title == "" {
		title = "Chart"
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	b.WriteString(`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	b.WriteString(`<c:chart><c:title><c:tx><c:rich><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>` + esc(title) + `</a:t></a:r></a:p></c:rich></c:tx><c:layout/><c:overlay val="0"/></c:title>`)
	b.WriteString(`<c:plotArea><c:layout/>`)
	if isLine {
		b.WriteString(`<c:lineChart><c:grouping val="standard"/><c:varyColors val="0"/>`)
	} else {
		b.WriteString(`<c:barChart><c:barDir val="col"/><c:grouping val="clustered"/><c:varyColors val="0"/>`)
	}
	for i, se := range series {
		nm := se.Name
		if nm == "" {
			nm = fmt.Sprintf("Series %d", i+1)
		}
		color := chartPalette[i%len(chartPalette)]
		fmt.Fprintf(&b, `<c:ser><c:idx val="%d"/><c:order val="%d"/><c:tx><c:v>%s</c:v></c:tx>`, i, i, esc(nm))
		fmt.Fprintf(&b, `<c:spPr><a:solidFill><a:srgbClr val="%s"/></a:solidFill></c:spPr>`, color)
		// categories
		fmt.Fprintf(&b, `<c:cat><c:strLit><c:ptCount val="%d"/>`, len(labels))
		for li, lb := range labels {
			fmt.Fprintf(&b, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, li, esc(lb))
		}
		b.WriteString(`</c:strLit></c:cat>`)
		// values
		fmt.Fprintf(&b, `<c:val><c:numLit><c:formatCode>General</c:formatCode><c:ptCount val="%d"/>`, len(se.Values))
		for vi, v := range se.Values {
			fmt.Fprintf(&b, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, vi, numStr(v))
		}
		b.WriteString(`</c:numLit></c:val></c:ser>`)
	}
	b.WriteString(`<c:axId val="1"/><c:axId val="2"/>`)
	if isLine {
		b.WriteString(`</c:lineChart>`)
	} else {
		b.WriteString(`</c:barChart>`)
	}
	b.WriteString(`<c:catAx><c:axId val="1"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:axPos val="b"/><c:tickLblPos val="nextTo"/><c:crossAx val="2"/><c:lblAlgn val="ctr"/><c:lblOffset val="100"/></c:catAx>`)
	b.WriteString(`<c:valAx><c:axId val="2"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:axPos val="l"/><c:majorGridlines/><c:numFmt formatCode="General" sourceLinked="1"/><c:tickLblPos val="nextTo"/><c:crossAx val="1"/><c:crosses val="autoZero"/></c:valAx>`)
	b.WriteString(`</c:plotArea>`)
	b.WriteString(`<c:legend><c:legendPos val="b"/><c:layout/><c:overlay val="0"/></c:legend>`)
	b.WriteString(`<c:plotVisOnly val="1"/></c:chart>`)
	b.WriteString(`</c:chartSpace>`)
	return b.String()
}

func pieChartXML(title string, labels []string, series []oir.ChartSeries) string {
	if title == "" {
		title = "Chart"
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	b.WriteString(`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	b.WriteString(`<c:chart><c:title><c:tx><c:rich><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>` + esc(title) + `</a:t></a:r></a:p></c:rich></c:tx><c:layout/><c:overlay val="0"/></c:title>`)
	b.WriteString(`<c:plotArea><c:layout/><c:pieChart><c:varyColors val="1"/>`)
	for i, se := range series {
		nm := se.Name
		if nm == "" {
			nm = fmt.Sprintf("Series %d", i+1)
		}
		fmt.Fprintf(&b, `<c:ser><c:idx val="%d"/><c:order val="%d"/><c:tx><c:v>%s</c:v></c:tx>`, i, i, esc(nm))
		fmt.Fprintf(&b, `<c:cat><c:strLit><c:ptCount val="%d"/>`, len(labels))
		for li, lb := range labels {
			fmt.Fprintf(&b, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, li, esc(lb))
		}
		b.WriteString(`</c:strLit></c:cat>`)
		fmt.Fprintf(&b, `<c:val><c:numLit><c:formatCode>General</c:formatCode><c:ptCount val="%d"/>`, len(se.Values))
		for vi, v := range se.Values {
			fmt.Fprintf(&b, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, vi, numStr(v))
		}
		b.WriteString(`</c:numLit></c:val></c:ser>`)
	}
	b.WriteString(`</c:pieChart></c:plotArea>`)
	b.WriteString(`<c:legend><c:legendPos val="b"/><c:layout/><c:overlay val="0"/></c:legend>`)
	b.WriteString(`<c:plotVisOnly val="1"/></c:chart>`)
	b.WriteString(`</c:chartSpace>`)
	return b.String()
}
