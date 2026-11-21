package telegram

import (
	"bytes"
	"io"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/renderer"
	"github.com/yuin/goldmark/v2/renderer/html"
)

func renderParagraph(
	w io.Writer,
	source []byte,
	n ast.Node,
	entering bool,
	rc renderer.Context,
) (ast.WalkStatus, error) {
	if entering || (n.Parent() != nil && n.Parent().Kind() == ast.KindListItem) {
		return ast.WalkContinue, nil
	}

	_, err := io.WriteString(w, "\n\n")
	return ast.WalkContinue, err
}

func renderThematicBreak(
	w io.Writer,
	source []byte,
	n ast.Node,
	entering bool,
	rc renderer.Context,
) (ast.WalkStatus, error) {
	if entering {
		_, err := io.WriteString(w, "──────────\n\n")
		return ast.WalkContinue, err
	}
	return ast.WalkContinue, nil
}

func renderHeading(w io.Writer, source []byte, n ast.Node, entering bool, rc renderer.Context) (ast.WalkStatus, error) {
	text := "<b>"
	if !entering {
		text = "</b>\n\n"
	}
	_, err := io.WriteString(w, text)
	return ast.WalkContinue, err
}

func renderList(w io.Writer, source []byte, n ast.Node, entering bool, rc renderer.Context) (ast.WalkStatus, error) {
	// Start nested lists on a new line; list items supply the other line breaks.
	if entering && n.Parent() != nil && n.Parent().Kind() == ast.KindListItem {
		_, err := io.WriteString(w, "\n")
		return ast.WalkContinue, err
	}
	return ast.WalkContinue, nil
}

func renderListItem(w io.Writer, source []byte, n ast.Node, entering bool, rc renderer.Context) (ast.WalkStatus, error) {
	if !entering {
		// A nested list already ends its final item with a newline.
		if last := n.LastChild(); last != nil && last.Kind() == ast.KindList {
			return ast.WalkContinue, nil
		}
		_, err := io.WriteString(w, "\n")
		return ast.WalkContinue, err
	}
	depth := 0
	for parent := n.Parent().Parent(); parent != nil; parent = parent.Parent() {
		if parent.Kind() == ast.KindListItem {
			depth++
		}
	}
	marker := "• "
	if list := n.Parent().(*ast.List); list.IsOrdered() {
		number := list.Start
		for sibling := n.PreviousSibling(); sibling != nil; sibling = sibling.PreviousSibling() {
			number++
		}
		marker = strconv.Itoa(number) + ". "
	}
	_, err := io.WriteString(w, strings.Repeat("  ", depth)+marker)
	return ast.WalkContinue, err
}

func renderText(w io.Writer, source []byte, n ast.Node, entering bool, rc renderer.Context) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	text := n.(*ast.Text)
	if _, err := text.Value.WriteTo(html.ContextTextWriter(rc), source); err != nil {
		return ast.WalkStop, err
	}
	if text.HardLineBreak() || text.SoftLineBreak() {
		_, err := io.WriteString(w, "\n")
		return ast.WalkContinue, err
	}
	return ast.WalkContinue, nil
}

// Render image descriptions as text, without unsupported img tags.
func renderImage(w io.Writer, source []byte, n ast.Node, entering bool, rc renderer.Context) (ast.WalkStatus, error) {
	return ast.WalkContinue, nil
}

func MarkdownToHTML(markdown string) (string, error) {
	source := []byte(markdown)

	p := parser.New()
	r := html.New(
		html.WithNodeRenderer(ast.KindHeading, html.NodeRendererFunc(renderHeading)),
		html.WithNodeRenderer(ast.KindList, html.NodeRendererFunc(renderList)),
		html.WithNodeRenderer(ast.KindListItem, html.NodeRendererFunc(renderListItem)),
		html.WithNodeRenderer(ast.KindText, html.NodeRendererFunc(renderText)),
		html.WithNodeRenderer(ast.KindImage, html.NodeRendererFunc(renderImage)),
		html.WithNodeRenderer(
			ast.KindParagraph,
			html.NodeRendererFunc(renderParagraph),
		),
		html.WithNodeRenderer(
			ast.KindThematicBreak,
			html.NodeRendererFunc(renderThematicBreak),
		),
	)

	document := p.Parse(source)

	var output bytes.Buffer
	if err := r.Render(&output, source, document); err != nil {
		return "", err
	}

	return strings.TrimSpace(output.String()), nil
}
