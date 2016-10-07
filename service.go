package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

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

	w := &worker{
		db:   newMemoryDatabase(),
		reqs: make(chan string, 10),
	}

	r := mux.NewRouter()
	r.HandleFunc("/queue/github.com/{user:[a-zA-Z-_]+}/{repo:[a-zA-Z-_]+}", w.queueRequest)
	r.HandleFunc("/results/github.com/{user:[a-zA-Z-_]+}/{repo:[a-zA-Z-_]+}", serveResults(w.db))

	go w.run()

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
		r, ok := db.load(path)
		if !ok {
			resp.WriteHeader(http.StatusNotFound)
			return
		}

		if r.Results == "" {
			resp.WriteHeader(http.StatusNotFound)
			return
		}

		res := map[string]interface{}{}
		err := json.Unmarshal([]byte(r.Results), &res)
		if err != nil {
			logger.Printf("invalid JSON document at path %s", path)
			resp.WriteHeader(http.StatusInternalServerError)
			return
		}

		raw, _ := json.Marshal(map[string]interface{}{"results": res})
		resp.Write(raw)
	}
}
