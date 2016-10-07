package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	gas "github.com/HewlettPackard/gas/core"
	"github.com/go-errors/errors"
	"github.com/gorilla/mux"
)

type worker struct {
	db   database
	reqs chan string
}

func (w *worker) queueRequest(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	user := vars["user"]
	repo := vars["repo"]

	select {
	case w.reqs <- fmt.Sprintf("%s/%s", user, repo):
		resp.WriteHeader(http.StatusAccepted)
	case <-time.After(100 * time.Millisecond):
		resp.WriteHeader(http.StatusServiceUnavailable)
	}
}

func (w *worker) run() {
	for repo := range w.reqs {
		logger.Printf("processing request for repository: %s", repo)
		out, err := w.process(repo)
		logError("worker error", err)

		r := results{
			Path: "github.com/" + repo,
			Time: time.Now().Unix(),
		}

		if out != nil && err == nil {
			res, _ := json.Marshal(out)
			r.Results = string(res)
		}

		if err != nil {
			r.Error = err.Error()
		}

		w.db.store(r)
	}
}

func (w *worker) process(repo string) (*gas.Analyzer, error) {
	defer func() {
		logError(fmt.Sprintf("panic processing %s", repo), recover())
	}()

	analyzer := buildAnalyzer()
	url := fmt.Sprintf("http://api.github.com/repos/%s/tarball", repo)

	dir, err := ioutil.TempDir("", "gas-web")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	res, err := http.Get(url)
	if err != nil {
		return nil, errors.WrapPrefix(err, fmt.Sprintf("unable to process %s", repo), 0)
	}

	if res.StatusCode != 200 {
		return nil, errors.Errorf("unable to process %s (%s)", repo, res.Status)
	}

	unzipped, err := gzip.NewReader(res.Body)
	if err != nil {
		return nil, errors.WrapPrefix(err, fmt.Sprintf("unable to process %s", repo), 0)
	}

	tar := tar.NewReader(unzipped)
	for i := 0; i < archiveFileLimit; i++ {
		header, err := tar.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, errors.WrapPrefix(err, fmt.Sprintf("unable to process %s", repo), 0)
		}

		path := filepath.Join(dir, header.Name)
		info := header.FileInfo()
		if info.IsDir() {
			if err = os.MkdirAll(path, info.Mode()); err != nil {
				return nil, err
			}
			continue
		}

		if !info.Mode().IsRegular() ||
			!strings.HasSuffix(header.Name, ".go") ||
			strings.Contains(header.Name, "vendor/") ||
			strings.HasSuffix(header.Name, "_test.go") {
			continue
		}

		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return nil, errors.WrapPrefix(err, fmt.Sprintf("unable to process %s", repo), 0)
		}
		defer file.Close()

		_, err = io.Copy(file, tar)
		if err != nil {
			return nil, err
		}

		analyzer.Process(path)
		if err != nil {
			return nil, errors.WrapPrefix(err, fmt.Sprintf("unable to process %s", repo), 0)
		}

		os.Remove(path)
	}

	for i, issue := range analyzer.Issues {
		issue.File = strings.SplitN(strings.Replace(issue.File, dir, "", -1), "/", 3)[2]
		analyzer.Issues[i] = issue
	}

	logger.Printf("done processing %s", repo)
	return analyzer, nil
}
