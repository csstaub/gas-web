package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"runtime"
	"time"

	"bitbucket.org/liamstask/goose/lib/goose"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

const (
	archiveFileLimit = 10000
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
	r.HandleFunc("/queue/github.com/{user:[a-zA-Z-0-9-_.]+}/{repo:[a-zA-Z0-9-_.]+}", h.HandleFunc(w.queueRequest)).Methods("POST")
	r.HandleFunc("/results/github.com/{user:[a-zA-Z0-9-_.]+}/{repo:[a-zA-Z0-9-_.]+}", h.HandleFunc(serveResults(w.db))).Methods("GET")

	r.PathPrefix("/").Handler(h.Handler(http.FileServer(http.Dir("assets/dist"))))

	for i := 0; i < runtime.NumCPU(); i++ {
		go w.run()
	}

	logger.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, cors.Default().Handler(r)); err != nil {
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

func serveResults(db database) func(resp http.ResponseWriter, req *http.Request) {
	return func(resp http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		user := vars["user"]
		repo := vars["repo"]

		path := fmt.Sprintf("github.com/%s/%s", user, repo)
		locked, err := db.isLocked(path)
		if err != nil {
			resp.WriteHeader(http.StatusInternalServerError)
		}

		t, _, r, err := db.fetchResults(path)
		if err != nil {
			if !locked {
				resp.WriteHeader(http.StatusNotFound)
			} else {
				resp.Header().Set("Content-Type", "application/json")
				raw, _ := json.Marshal(map[string]interface{}{
					"time":       time.Now(),
					"repo":       fmt.Sprintf("github.com/%s/%s", user, repo),
					"processing": true,
				})
				resp.Write(raw)
			}
			return
		}

		res := map[string]interface{}{}
		err = json.Unmarshal([]byte(r), &res)
		if err != nil {
			logger.Printf("invalid JSON document at path %s", path)
			resp.WriteHeader(http.StatusInternalServerError)
			return
		}

		resp.Header().Set("Content-Type", "application/json")
		resp.Header().Set("Cache-Control", fmt.Sprintf("max-age:%d", 1*time.Hour/time.Second))

		raw, _ := json.Marshal(map[string]interface{}{
			"time":       t,
			"repo":       fmt.Sprintf("github.com/%s/%s", user, repo),
			"results":    res,
			"processing": locked,
		})
		resp.Write(raw)
	}
}
