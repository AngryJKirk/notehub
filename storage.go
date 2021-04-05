package main

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"html/template"
	"math"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

const (
	idLength       = 5
	fraudThreshold = 7
)

var (
	rexpNoteID = regexp.MustCompile("[a-z0-9]+")
	rexpLink   = regexp.MustCompile("(ht|f)tps?://[^\\s]+")
)

type Note struct {
	ID, Title, Text, Password, DeprecatedPassword, Encoded, Name string
	Published, Edited                                      time.Time
	Views                                                  int
	Content, Ads                                           template.HTML
}

func (n *Note) Fraud() bool {
	res := rexpLink.FindAllString(n.Text, -1)
	if len(res) < 3 {
		return false
	}
	stripped := rexpLink.ReplaceAllString(n.Text, "")
	l1 := len(n.Text)
	l2 := len(stripped)
	return n.Views > 150 &&
		int(math.Ceil(100*float64(l1-l2)/float64(l1))) > fraudThreshold
}

func save(c echo.Context, db *sql.DB, n *Note) (*Note, error) {
	if n.Password != "" {
		clean := n.Password
		n.Password = fmt.Sprintf("%x", sha256.Sum256([]byte(n.Password)))
		h := md5.New()
		h.Write([]byte(clean))
		n.DeprecatedPassword = fmt.Sprintf("%x", h.Sum(nil))
	}
	if n.ID == "" {
		return insert(c, db, n)
	}
	if !rexpNoteID.Match([]byte(n.ID)) {
		return nil, errorBadRequest
	}
	return update(c, db, n)
}

func update(c echo.Context, db *sql.DB, n *Note) (*Note, error) {
	c.Logger().Debugf("updating note %s", n.ID)
	if n.Password == "" {
		return nil, errorBadRequest
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	s := "update notes set (text, edited, password) = (?, ?, ?) where id = ? and (password = ? or password = ?)"
	if n.Text == "" {
		s = "delete from notes where id = ? and (password = ? or password = ?)"
	}
	stmt, _ := tx.Prepare(s)
	defer stmt.Close()
	var res sql.Result
	if n.Text == "" {
		res, err = stmt.Exec(n.ID, n.Password, n.DeprecatedPassword)
	} else {
		res, err = stmt.Exec(n.Text, time.Now(), n.Password, n.ID, n.Password, n.DeprecatedPassword)
	}
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	rows, err := res.RowsAffected()
	if rows != 1 {
		tx.Rollback()
		return nil, errorUnathorised
	}
	c.Logger().Debugf("updating note %s (deletion: %t); committing transaction", n.ID, n.Text == "")
	return n, tx.Commit()
}

func insert(c echo.Context, db *sql.DB, n *Note) (*Note, error) {
	c.Logger().Debug("inserting new note")
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	stmt, _ := tx.Prepare("insert into notes(id, text, password) values(?, ?, ?)")
	defer stmt.Close()
	id := n.Name
	if id == "" {
		id = randId()
	}
	_, err = stmt.Exec(id, n.Text, n.Password)
	if err != nil {
		tx.Rollback()
		if strings.HasPrefix(err.Error(), "UNIQUE constraint failed") {
			c.Logger().Infof("collision on id %s", id)
			return nil, errorNameExists
		}
		return nil, err
	}
	n.ID = id
	c.Logger().Debugf("inserting new note %s; commiting transaction", n.ID)
	return n, tx.Commit()
}

func randId() string {
	buf := bytes.NewBuffer([]byte{})
	for i := 0; i < idLength; i++ {
		b := '0'
		z := rand.Intn(36)
		if z > 9 {
			b = 'a'
			z -= 10
		}
		buf.WriteRune(rune(z) + b)
	}
	return buf.String()
}

func load(c echo.Context, db *sql.DB) (*Note, int) {
	q := c.Param("id")
	if !rexpNoteID.Match([]byte(q)) {
		code := http.StatusNotFound
		return nil, code
	}
	c.Logger().Debugf("loading note %s", q)
	stmt, _ := db.Prepare("select * from notes where id = ?")
	defer stmt.Close()
	row := stmt.QueryRow(q)
	var id, text, password string
	var published time.Time
	var editedVal interface{}
	var views int
	if err := row.Scan(&id, &text, &published, &editedVal, &password, &views); err != nil {
		code := http.StatusNotFound
		return nil, code
	}
	n := &Note{
		ID:        id,
		Text:      text,
		Views:     views,
		Published: published,
	}
	if editedVal != nil {
		n.Edited = editedVal.(time.Time)
	}
	n.prepare()
	return n, http.StatusOK
}

func loadAll(c echo.Context, db *sql.DB) ([]Note, int) {

	c.Logger().Debug("loading notes")
	stmt, _ := db.Prepare("select * from notes")
	defer stmt.Close()
	rows, _ := stmt.Query()
	var notes []Note
	for rows.Next() {
		var id, text, password string
		var published time.Time
		var editedVal interface{}
		var views int
		if err := rows.Scan(&id, &text, &published, &editedVal, &password, &views); err != nil {
			code := http.StatusNotFound
			return nil, code
		}
		n := &Note{
			ID:        id,
			Text:      text,
			Views:     views,
			Published: published,
		}
		if editedVal != nil {
			n.Edited = editedVal.(time.Time)
		}
		n.prepare()
		notes = append(notes, *n)
	}


	return notes, http.StatusOK
}
