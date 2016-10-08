package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

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

func main() {
	addr := getAddr()

	db, err := newSQLDatabase()
	if err != nil {
		logError("unable to connect to database", err)
		os.Exit(1)
	}

	w := &worker{
		db:   db,
		reqs: make(chan string, 10),
	}

	r := mux.NewRouter()
	r.HandleFunc("/queue/github.com/{user:[a-zA-Z-_]+}/{repo:[a-zA-Z-_]+}", w.queueRequest)
	r.HandleFunc("/results/github.com/{user:[a-zA-Z-_]+}/{repo:[a-zA-Z-_]+}", serveResults(w.db))

	for i := 0; i < runtime.NumCPU(); i++ {
		go w.run()
	}

	logger.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, cors.Default().Handler(r)); err != nil {
		logger.Fatal(err)
	}
}

func serveResults(db database) func(resp http.ResponseWriter, req *http.Request) {
	return func(resp http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		user := vars["user"]
		repo := vars["repo"]

		path := fmt.Sprintf("github.com/%s/%s", user, repo)
		t, r, err := db.fetchResults(path)
		if err != nil {
			locked, err := db.isLocked(path)
			if err != nil {
				resp.WriteHeader(http.StatusInternalServerError)
			} else if !locked {
				resp.WriteHeader(http.StatusNotFound)
			} else {
				raw, _ := json.Marshal(map[string]interface{}{
					"time":       time.Now(),
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

		raw, _ := json.Marshal(map[string]interface{}{
			"time":    t,
			"results": res,
		})
		resp.Write(raw)
	}
}
