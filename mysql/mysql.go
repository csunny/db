/*
  Copyright (c) 2012-2013 José Carlos Nieto, http://xiam.menteslibres.org/

  Permission is hereby granted, free of charge, to any person obtaining
  a copy of this software and associated documentation files (the
  "Software"), to deal in the Software without restriction, including
  without limitation the rights to use, copy, modify, merge, publish,
  distribute, sublicense, and/or sell copies of the Software, and to
  permit persons to whom the Software is furnished to do so, subject to
  the following conditions:

  The above copyright notice and this permission notice shall be
  included in all copies or substantial portions of the Software.

  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
  EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
  MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
  NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
  LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
  OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
  WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/

package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/Go-SQL-Driver/MySQL"
	"github.com/gosexy/db"
	"reflect"
	"regexp"
	"strings"
)

var Debug = false

// Format for saving dates.
const DateFormat = "2006-01-02 15:04:05.000"

// Format for saving times.
const TimeFormat = "%d:%02d:%02d.%03d"

var columnPattern = regexp.MustCompile(`^([a-z]+)\(?([0-9,]+)?\)?\s?([a-z]*)?`)

func init() {
	db.Register("mysql", &Source{})
}

/*
	Driver's session data.
*/
type Source struct {
	session     *sql.DB
	config      db.DataSource
	collections map[string]db.Collection
}

// Mysql table/collection.
type Table struct {
	parent *Source
	name   string
	types  map[string]reflect.Kind
}

type sqlQuery struct {
	Query   []string
	SqlArgs []interface{}
}

func sqlCompile(terms []interface{}) *sqlQuery {
	q := &sqlQuery{}

	q.Query = []string{}

	for _, term := range terms {
		switch t := term.(type) {
		case string:
			q.Query = append(q.Query, t)
		case db.SqlArgs:
			for _, arg := range t {
				q.SqlArgs = append(q.SqlArgs, arg)
			}
		case db.SqlValues:
			args := make([]string, len(t))
			for i, arg := range t {
				args[i] = "?"
				q.SqlArgs = append(q.SqlArgs, arg)
			}
			q.Query = append(q.Query, "("+strings.Join(args, ", ")+")")
		}
	}

	return q
}

func sqlFields(names []string) string {
	for i, _ := range names {
		names[i] = strings.Replace(names[i], "`", "``", -1)
	}
	return "(`" + strings.Join(names, "`, `") + "`)"
}

func sqlValues(values []string) db.SqlValues {
	ret := make(db.SqlValues, len(values))
	for i, _ := range values {
		ret[i] = values[i]
	}
	return ret
}

func sqlTable(name string) string {
	return name
}

/*
	Returns the name of the database.
*/
func (self *Source) Name() string {
	return self.config.Database
}

// Wraps sql.DB.QueryRow
func (self *Source) doQueryRow(terms ...interface{}) (*sql.Row, error) {
	if self.session == nil {
		return nil, errors.New("You're currently not connected.")
	}

	chunks := sqlCompile(terms)

	query := strings.Join(chunks.Query, " ")

	if Debug == true {
		fmt.Printf("Q: %s\n", query)
		fmt.Printf("A: %v\n", chunks.SqlArgs)
	}

	return self.session.QueryRow(query, chunks.SqlArgs...), nil
}

// Wraps sql.DB.Query
func (self *Source) doQuery(terms ...interface{}) (*sql.Rows, error) {
	if self.session == nil {
		return nil, fmt.Errorf("You're currently not connected.")
	}

	chunks := sqlCompile(terms)

	query := strings.Join(chunks.Query, " ")

	if Debug == true {
		fmt.Printf("Q: %s\n", query)
		fmt.Printf("A: %v\n", chunks.SqlArgs)
	}

	return self.session.Query(query, chunks.SqlArgs...)
}

// Wraps sql.DB.Exec
func (self *Source) doExec(terms ...interface{}) (sql.Result, error) {
	if self.session == nil {
		return nil, fmt.Errorf("You're currently not connected.")
	}

	chunks := sqlCompile(terms)

	query := strings.Join(chunks.Query, " ")

	if Debug == true {
		fmt.Printf("Q: %s\n", query)
		fmt.Printf("A: %v\n", chunks.SqlArgs)
	}

	return self.session.Exec(query, chunks.SqlArgs...)
}

/*
	Closes a database session.
*/
func (self *Source) Close() error {
	if self.session != nil {
		return self.session.Close()
	}
	return nil
}

/*
	Configures and returns a database session.
*/
func (self *Source) Setup(config db.DataSource) error {
	self.config = config
	self.collections = make(map[string]db.Collection)
	return self.Open()
}

/*
	Tries to open a database connection.
*/
func (self *Source) Open() error {
	var err error

	if self.config.Host == "" {
		if self.config.Socket == "" {
			self.config.Host = "127.0.0.1"
		}
	}

	if self.config.Port == 0 {
		self.config.Port = 3306
	}

	if self.config.Database == "" {
		return fmt.Errorf("Database name is required.")
	}

	if self.config.Socket != "" && self.config.Host != "" {
		return errors.New("Socket or Host are mutually exclusive.")
	}

	if self.config.Charset == "" {
		self.config.Charset = "utf8"
	}

	var conn string

	if self.config.Host != "" {
		conn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s", self.config.User, self.config.Password, self.config.Host, self.config.Port, self.config.Database, self.config.Charset)
	} else if self.config.Socket != "" {
		conn = fmt.Sprintf("%s:%s@unix(%s)/%s?charset=%s", self.config.User, self.config.Password, self.config.Socket, self.config.Database, self.config.Charset)
	}

	self.session, err = sql.Open("mysql", conn)

	if err != nil {
		return err
	}

	return nil
}

/*
	Changes the active database.
*/
func (self *Source) Use(database string) error {
	self.config.Database = database
	_, err := self.session.Exec(fmt.Sprintf("USE %s", database))
	return err
}

/*
	Drops the currently active database.
*/
func (self *Source) Drop() error {
	_, err := self.session.Exec(fmt.Sprintf("DROP DATABASE %s", self.config.Database))
	return err
}

/*
	Returns a *sql.DB object that represents an internal session.
*/
func (self *Source) Driver() interface{} {
	return self.session
}

/*
	Returns a list of all tables in the current database.
*/
func (self *Source) Collections() []string {
	var collections []string
	var collection string

	rows, err := self.session.Query("SHOW TABLES")

	if err == nil {
		for rows.Next() {
			rows.Scan(&collection)
			collections = append(collections, collection)
		}
	} else {
		panic(err)
	}

	return collections
}

/*
	Returns a collection that must exists or panics.
*/
func (self *Source) ExistentCollection(name string) db.Collection {
	col, err := self.Collection(name)
	if err != nil {
		panic(err.Error())
	}
	return col
}

/*
	Returns a table struct by name.
*/
func (self *Source) Collection(name string) (db.Collection, error) {

	if col, ok := self.collections[name]; ok == true {
		return col, nil
	}

	table := &Table{}

	table.parent = self
	table.name = name

	// Table exists?
	if table.Exists() == false {
		return table, fmt.Errorf("Table %s does not exists.", name)
	}

	// Fetching table datatypes and mapping to internal gotypes.
	rows, err := table.parent.doQuery(
		"SHOW COLUMNS FROM",
		table.Name(),
	)

	if err != nil {
		return table, err
	}

	columns := []struct {
		Field string
		Type  string
	}{}

	err = table.fetchRows(&columns, rows)

	if err != nil {
		return nil, err
	}

	table.types = make(map[string]reflect.Kind, len(columns))

	for _, column := range columns {

		column.Field = strings.ToLower(column.Field)
		column.Type = strings.ToLower(column.Type)

		results := columnPattern.FindStringSubmatch(column.Type)

		// Default properties.
		dextra := ""
		dtype := "varchar"

		dtype = results[1]

		if len(results) > 3 {
			dextra = results[3]
		}

		ctype := reflect.String

		// Guessing datatypes.
		switch dtype {
		case "tinyint", "smallint", "mediumint", "int", "bigint":
			if dextra == "unsigned" {
				ctype = reflect.Uint64
			} else {
				ctype = reflect.Int64
			}
		case "decimal", "float", "double":
			ctype = reflect.Float64
		}

		table.types[column.Field] = ctype
	}

	self.collections[name] = table

	return table, nil
}
