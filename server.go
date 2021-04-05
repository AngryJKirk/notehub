package main

import (
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"database/sql"

	_ "github.com/mattn/go-sqlite3"

	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"
)


type Template struct{ templates *template.Template }

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func main() {
	e := echo.New()
	e.Logger.SetLevel(log.DEBUG)

	db, err := sql.Open("sqlite3", "./data/database.sqlite")
	if err != nil {
		e.Logger.Error(err)
	}
	defer db.Close()

	adsFName := os.Getenv("ADS")
	var ads template.HTML
	if adsFName != "" {
		data, err := ioutil.ReadFile(adsFName)
		if err != nil {
			e.Logger.Errorf("couldn't read file %s: %v", adsFName, err)
		}
		ads = mdTmplHTML(data)
	}

	go flushStatsLoop(e.Logger, db)

	e.Renderer = &Template{templates: template.Must(template.ParseGlob("assets/templates/*.html"))}

	e.File("/favicon.ico", "assets/public/favicon.ico")
	e.File("/robots.txt", "assets/public/robots.txt")
	e.File("/style.css", "assets/public/style.css")
	e.File("/new.js", "assets/public/new.js")
	e.File("/note.js", "assets/public/note.js")
	e.File("/index.html", "assets/public/index.html")
	e.File("/", "assets/public/index.html")
	e.File("/Markdown.Converter.js", "assets/public/editor/Markdown.Converter.js")
	e.File("/Markdown.Editor.js", "assets/public/editor/Markdown.Editor.js")
	e.File("/Markdown.Extra.js", "assets/public/editor/Markdown.Extra.js")
	e.File("/Markdown.Sanitizer.js", "assets/public/editor/Markdown.Sanitizer.js")
	e.File("/mathjax-editing_writing.js", "assets/public/editor/mathjax-editing_writing.js")
	e.File("/cmunrb.otf", "assets/public/editor/cmunrb.otf")
	e.File("/cmunrm.otf", "assets/public/editor/cmunrm.otf")
	e.File("/editor", "assets/public/editor/index.html")


	e.GET("/:id", func(c echo.Context) error {
		id := c.Param("id")
		n, code := load(c, db)
		if code != http.StatusOK {
			c.Logger().Errorf("/%s failed (code: %d)", id, code)
			return c.String(code, statuses[code])
		}
		defer incViews(n, db)
		n.Ads = ads
		c.Logger().Debugf("/%s delivered (fraud: %t)", id, n.Fraud())
		return c.Render(code, "Note", n)
	})

	e.GET("/:id/export", func(c echo.Context) error {
		id := c.Param("id")
		n, code := load(c, db)
		var content string
		if code == http.StatusOK {
			defer incViews(n, db)
			if n.Fraud() {
				code = http.StatusForbidden
				content = statuses[code]
				c.Logger().Warnf("/%s/export failed (code: %d)", id, code)
			} else {
				content = n.Text
				c.Logger().Debugf("/%s/export delivered", id)
			}
		}
		return c.String(code, content)
	})

	e.GET("/:id/stats", func(c echo.Context) error {
		id := c.Param("id")
		n, code := load(c, db)
		if code != http.StatusOK {
			c.Logger().Errorf("/%s/stats failed (code: %d)", id, code)
			return c.String(code, statuses[code])
		}
		stats := fmt.Sprintf("Published: %s\n    Views: %d", n.Published, n.Views)
		if !n.Edited.IsZero() {
			stats = fmt.Sprintf("Published: %s\n   Edited: %s\n    Views: %d", n.Published, n.Edited, n.Views)
		}
		return c.String(code, stats)
	})

	e.GET("/:id/edit", func(c echo.Context) error {
		id := c.Param("id")
		n, code := load(c, db)
		if code != http.StatusOK {
			c.Logger().Errorf("/%s/edit failed (code: %d)", id, code)
			return c.String(code, statuses[code])
		}
		c.Logger().Debugf("/%s/edit delivered", id)
		return c.Render(code, "Form", n)
	})

	e.POST("/:id/report", func(c echo.Context) error {
		report := c.FormValue("report")
		if report != "" {
			id := c.Param("id")
			c.Logger().Infof("note %s was reported: %s", id, report)
		}
		return c.NoContent(http.StatusNoContent)
	})

	e.GET("/new", func(c echo.Context) error {
		c.Logger().Debug("/new opened")
		return c.Render(http.StatusOK, "Form", nil)
	})

	e.GET("/list", func(c echo.Context) error {
		c.Logger().Debug("GET /list")
		notes, err := loadAll(c, db)
		if err != http.StatusOK {
			return c.String(err, "Error happened")
		}
		return c.Render(http.StatusOK, "List", notes)
	})

	type postResp struct {
		Success bool
		Payload string
	}

	e.POST("/", func(c echo.Context) error {
		c.Logger().Debug("POST /")
		id := c.FormValue("id")
		text := c.FormValue("text")
		n := &Note{
			ID:       id,
			Text:     text,
			Password: c.FormValue("password"),
			Name: c.FormValue("name"),
		}
		n, err = save(c, db, n)
		if err != nil {
			c.Logger().Error(err)
			code := http.StatusServiceUnavailable
			if err == errorUnathorised {
				code = http.StatusUnauthorized
			} else if err == errorBadRequest {
				code = http.StatusBadRequest
			} else if err == errorNameExists {
				code = http.StatusBadRequest
			}
			c.Logger().Errorf("POST / error: %d", code)
			return c.JSON(code, postResp{false, statuses[code] + ": " + err.Error()})
		}
		if id == "" {
			c.Logger().Infof("note %s created", n.ID)
			return c.JSON(http.StatusCreated, postResp{true, n.ID})
		} else if text == "" {
			c.Logger().Infof("note %s deleted", n.ID)
		} else {
			c.Logger().Infof("note %s updated", n.ID)
		}
		return c.JSON(http.StatusOK, postResp{true, n.ID})
	})

	s := &http.Server{
		Addr:         ":3000",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	e.Logger.Fatal(e.StartServer(s))
}
