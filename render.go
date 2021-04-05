package main

import (
	"encoding/base64"
	"errors"
	"html/template"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/golang-commonmark/markdown"
	"github.com/labstack/echo"
)

var (
	statuses = map[int]string{
		400: "Bad request",
		401: "Unauthorized",
		403: "Forbidden",
		404: "Not found",
		412: "Precondition failed",
		503: "Service unavailable",
	}

	rexpNewLine        = regexp.MustCompile("[\n\r]")
	rexpNonAlphaNum    = regexp.MustCompile("[`~!@#$%^&*_|+=?;:'\",.<>{}\\/]")
	rexpNoScriptIframe = regexp.MustCompile("(<.*?script.*?>.*?<.*?/.*?script.*?>|<.*?iframe.*?>|</.*?iframe.*?>)")

	errorUnathorised = errors.New("password is wrong")
	errorBadRequest  = errors.New("password is empty")
	errorNameExists  = errors.New("name exists")
)

func (n *Note) prepare() {
	fstLine := rexpNewLine.Split(n.Text, -1)[0]
	maxLength := 25
	if len(fstLine) < 25 {
		maxLength = len(fstLine)
	}
	n.Text = rexpNoScriptIframe.ReplaceAllString(n.Text, "")
	n.Title = strings.TrimSpace(rexpNonAlphaNum.ReplaceAllString(fstLine[:maxLength], ""))
	n.Content = mdTmplHTML([]byte(n.Text))
	if n.Fraud() {
		n.Encoded = base64.StdEncoding.EncodeToString([]byte(n.Content))
	}
}

var mdRenderer = markdown.New(markdown.HTML(true))

func mdTmplHTML(content []byte) template.HTML {
	return template.HTML(mdRenderer.RenderToString(content))
}

func md2html(c echo.Context, name string) (*Note, int) {
	path := "assets/markdown/" + name + ".md"
	mdContent, err := ioutil.ReadFile(path)
	if err != nil {
		c.Logger().Errorf("couldn't open markdown page %s: %v", path, err)
		code := http.StatusServiceUnavailable
		return nil, code
	}
	c.Logger().Debugf("rendering markdown page %s", name)
	return &Note{Title: name, Content: mdTmplHTML(mdContent)}, http.StatusOK
}
