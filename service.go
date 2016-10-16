// Copyright (c) 2016, Cedric Staub <css@css.bio>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"runtime"

	"bitbucket.org/liamstask/goose/lib/goose"

	"github.com/gorilla/mux"
)

const (
	archiveFileLimit = 5000
)

var (
	logger = log.New(os.Stderr, "", 0)
)

func init() {
	devNull, err := os.Open("/dev/null")
	if err != nil {
		panic(err)
	}
	os.Stdout = devNull
}

func getAddr() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return fmt.Sprintf(":%s", port)
}

type headerWrapper struct {
	handler func(resp http.ResponseWriter, req *http.Request)
	headers map[string]string
}

func (h headerWrapper) Handler(handler http.Handler) http.Handler {
	return headerWrapper{
		handler: handler.ServeHTTP,
		headers: h.headers,
	}
}

func (h headerWrapper) HandleFunc(handler func(resp http.ResponseWriter, req *http.Request)) func(resp http.ResponseWriter, req *http.Request) {
	return headerWrapper{
		handler: handler,
		headers: h.headers,
	}.ServeHTTP
}

func (h headerWrapper) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	for k, v := range h.headers {
		resp.Header().Set(k, v)
	}
	h.handler(resp, req)
}

func main() {
	addr := getAddr()

	db, err := newSQLDatabase()
	if err != nil {
		logError("unable to connect to database", err)
		os.Exit(1)
	}
	migrate(db.(*sqlDatabase).DB)

	w := &worker{
		db:   db,
		reqs: make(chan string, 10),
	}

	h := &headerWrapper{
		headers: map[string]string{
			"Content-Security-Policy": "default-src 'self' cdnjs.cloudflare.com;",
			"X-Content-Type-Options":  "nosniff",
			"X-Frame-Options":         "deny",
			"X-XSS-Protection":        "1; mode=block",
		}}

	r := mux.NewRouter()
	r.HandleFunc("/results/github.com/{user:[a-zA-Z0-9-_.]+}/{repo:[a-zA-Z0-9-_.]+}", h.HandleFunc(w.serveResults)).Methods("GET")

	r.PathPrefix("/").Handler(h.Handler(http.FileServer(http.Dir("assets/dist"))))

	for i := 0; i < runtime.NumCPU()*2; i++ {
		go w.run()
	}

	logger.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		logger.Fatal(err)
	}
}

func migrate(db *sql.DB) {
	migrations := path.Join("db/migrations")

	gooseConf := goose.DBConf{
		MigrationsDir: migrations,
		Env:           "gas-web",
		Driver: goose.DBDriver{
			Name:    "mysql",
			Import:  "github.com/go-sql-driver/mysql",
			Dialect: &goose.MySqlDialect{},
		},
	}

	desiredVersion, err := goose.GetMostRecentDBVersion(migrations)
	if err != nil {
		logger.Fatalf("unable to run migrations: %s", err)
	}

	err = goose.RunMigrationsOnDb(&gooseConf, migrations, desiredVersion, db)
	if err != nil {
		logger.Fatalf("unable to run migrations: %s", err)
	}

	logger.Printf("ran migrations up to version %d", desiredVersion)
}
