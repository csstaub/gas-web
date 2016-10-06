package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gorilla/mux"
	"github.com/guregu/dynamo"
	"github.com/rs/cors"
)

const (
	archiveFileLimit = 10000
)

var logger *log.Logger

func init() {
	logger = log.New(os.Stderr, "", 0)
}

type resultsHandler struct {
	db    *dynamo.DB
	table *dynamo.Table
}

type results struct {
	Path    string `dynamo:"path"`
	Results string `dynamo:"results"`
}

func getAddr() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return fmt.Sprintf(":%s", port)
}

func main() {
	config := &aws.Config{
		Region: aws.String("us-east-1"),
	}

	addr := getAddr()
	db := dynamo.New(session.New(), config)
	table := db.Table("gas-web-output")

	handler := &resultsHandler{
		db:    db,
		table: &table,
	}

	r := mux.NewRouter()

	h := http.StripPrefix("/results/", cors.Default().Handler(handler))
	r.PathPrefix("/results/").Methods("GET", "OPTIONS").Handler(h)

	logger.Printf("Listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		logger.Fatal(err)
	}
}

func (r *resultsHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	if !strings.HasPrefix(req.URL.Path, "github.com/") {
		resp.WriteHeader(http.StatusNotFound)
		return
	}

	repo := strings.Replace(req.URL.Path, "github.com/", "", 1)
	if strings.Count(repo, "/") > 1 {
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	results, err := r.process(repo)
	if err != nil {
		logger.Printf("error processing: %s", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	out, _ := json.Marshal(results)
	resp.Write(out)
}

func (r *resultsHandler) process(repo string) (interface{}, error) {
	dir, err := ioutil.TempDir("", "gas-web")
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("http://api.github.com/repos/%s/tarball", repo)

	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected response from github: %s", res.Status)
	}

	unzipped, err := gzip.NewReader(res.Body)
	if err != nil {
		return nil, err
	}

	tar := tar.NewReader(unzipped)
	for i := 0; i < archiveFileLimit; i++ {
		header, err := tar.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		logger.Printf("processing file: %s", header.Name)

		path := filepath.Join(dir, header.Name)
		info := header.FileInfo()
		if info.IsDir() {
			if err = os.MkdirAll(path, info.Mode()); err != nil {
				return nil, err
			}
			continue
		}

		if !strings.HasSuffix(header.Name, ".go") {
			continue
		}

		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return nil, err
		}
		defer file.Close()

		_, err = io.Copy(file, tar)
		if err != nil {
			return nil, err
		}
	}

	return "success", nil
}
