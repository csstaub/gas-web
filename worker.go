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
	uuid "github.com/satori/go.uuid"
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
	nodeID := uuid.NewV4().String()
	logger.Printf("running worker %s", nodeID)

	for repo := range w.reqs {
		// Process
		logger.Printf("node %s processing request for %s", nodeID, repo)
		out, err := w.process(nodeID, repo)
		logError(fmt.Sprintf("node %s worker error", nodeID), err)

		if out != nil && err == nil {
			path := fmt.Sprintf("github.com/%s", repo)
			res, _ := json.Marshal(out)
			err := w.db.storeResults(path, string(res))
			logError("unable to store results", err)
		}
	}
}

func (w *worker) process(nodeID, repo string) (*gas.Analyzer, error) {
	defer func() {
		logError(fmt.Sprintf("panic processing %s", repo), recover())
	}()

	path := fmt.Sprintf("github.com/%s", repo)
	t, _, err := w.db.fetchResults(path)
	if err == nil && t.Add(5*time.Minute).After(time.Now()) {
		logger.Printf("node %s skipping %s, results less than 5 minutes old", nodeID, repo)
		return nil, nil
	}

	// Acquire lock
	locked := time.Now()
	lock, err := w.db.lockPath(nodeID, path, 5*time.Minute)
	if err != nil {
		return nil, errors.WrapPrefix(err, "error acquiring lock", 0)
	}
	if lock == nil {
		logger.Printf("node %s skipping %s, already locked", nodeID, repo)
		return nil, nil
	}
	defer lock.unlock()

	analyzer := buildAnalyzer()
	url := fmt.Sprintf("http://api.github.com/repos/%s/tarball", repo)

	dir, err := ioutil.TempDir("", "gas-web")
	if err != nil {
		return nil, errors.WrapPrefix(err, fmt.Sprintf("unable to process %s", repo), 0)
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
		// Refresh lock every minute
		if time.Now().After(locked.Add(1 * time.Minute)) {
			err = lock.refresh()
			if err != nil {
				return nil, errors.WrapPrefix(err, "lost lock, aborting", 0)
			}
		}

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
				return nil, errors.WrapPrefix(err, fmt.Sprintf("unable to process %s", repo), 0)
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
			return nil, errors.WrapPrefix(err, fmt.Sprintf("unable to process %s", repo), 0)
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

	logger.Printf("node %s done processing %s", nodeID, repo)
	return analyzer, nil
}
