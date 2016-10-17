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
	"archive/tar"
	"compress/gzip"
	"database/sql"
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

var (
	errNotModified = errors.New("not modified")
	errNotFound    = errors.New("not found")
)

type worker struct {
	db   database
	reqs chan string
}

func (w *worker) queueRequest(user, repo string) bool {
	select {
	case w.reqs <- fmt.Sprintf("%s/%s", user, repo):
		return true
	case <-time.After(100 * time.Millisecond):
		return false
	}
}

func (w *worker) run() {
	nodeID := uuid.NewV4().String()
	logger.Printf("running worker %s", nodeID)

	for repo := range w.reqs {
		// Process
		logger.Printf("node %s processing request for %s", nodeID, repo)
		out, etag, err := w.process(nodeID, repo)

		if err != errNotFound && err != errNotModified {
			logError(fmt.Sprintf("node %s worker error", nodeID), err)
		}

		path := fmt.Sprintf("github.com/%s", repo)
		if err == errNotFound {
			err := w.db.storeResults(path, "", "", true)
			logError("unable to store results", err)
		} else if err == errNotModified {
			err := w.db.updateTimestamp(path)
			logError("unable to update timestamp", err)
		} else if out != nil && err == nil {
			res, _ := json.Marshal(out)
			err := w.db.storeResults(path, etag, string(res), false)
			logError("unable to store results", err)
		}
	}
}

func (w *worker) process(nodeID, repo string) (*gas.Analyzer, string, error) {
	defer func() {
		logError(fmt.Sprintf("panic processing %s", repo), recover())
	}()

	path := fmt.Sprintf("github.com/%s", repo)
	t, etag, _, _, err := w.db.fetchResults(path)
	if err == nil && t.Add(1*time.Hour).After(time.Now()) {
		logger.Printf("node %s skipping %s, results less than 1 hour old", nodeID, repo)
		return nil, "", nil
	}

	// Acquire lock
	locked := time.Now()
	lock, err := w.db.lockPath(nodeID, path, 5*time.Minute)
	if err != nil {
		return nil, "", errors.WrapPrefix(err, "error acquiring lock", 0)
	}
	if lock == nil {
		logger.Printf("node %s skipping %s, already locked", nodeID, repo)
		return nil, "", nil
	}
	defer lock.unlock()

	analyzer := buildAnalyzer()
	url := fmt.Sprintf("https://api.github.com/repos/%s/tarball", repo)

	dir, err := ioutil.TempDir("", "gas-web")
	if err != nil {
		return nil, "", errors.WrapPrefix(err, fmt.Sprintf("unable to process %s", repo), 0)
	}
	defer os.RemoveAll(dir)

	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return nil, "", errors.WrapPrefix(err, fmt.Sprintf("unable to process %s", repo), 0)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", errors.WrapPrefix(err, fmt.Sprintf("unable to process %s", repo), 0)
	}

	io.Copy(ioutil.Discard, res.Body)
	res.Body.Close()

	if res.StatusCode == 304 {
		// Not modified since last fetch
		logger.Printf("node %s skipping %s, not modified since last fetch", nodeID, repo)
		return nil, "", errNotModified
	}

	if res.StatusCode == 404 {
		return nil, "", errNotFound
	}

	if res.StatusCode != 200 {
		return nil, "", errors.Errorf("unable to process %s (%s)", repo, res.Status)
	}

	serverTag := res.Header.Get("ETag")
	if serverTag != "" && etag == serverTag {
		logger.Printf("node %s skipping %s, not modified since last fetch", nodeID, repo)
		return nil, "", errNotModified
	}

	req, err = http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", errors.WrapPrefix(err, fmt.Sprintf("unable to process %s", repo), 0)
	}

	if etag != "" {
		req.Header.Add("If-None-Match", etag)
	}

	res, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", errors.WrapPrefix(err, fmt.Sprintf("unable to process %s", repo), 0)
	}
	defer io.Copy(ioutil.Discard, res.Body)
	defer res.Body.Close()

	if res.StatusCode == 304 {
		// Not modified since last fetch
		logger.Printf("node %s skipping %s, not modified since last fetch", nodeID, repo)
		return nil, "", errNotModified
	}

	if res.StatusCode == 404 {
		return nil, "", errNotFound
	}

	if res.StatusCode != 200 {
		return nil, "", errors.Errorf("unable to process %s (%s)", repo, res.Status)
	}

	unzipped, err := gzip.NewReader(res.Body)
	if err != nil {
		return nil, "", errors.WrapPrefix(err, fmt.Sprintf("unable to process %s", repo), 0)
	}

	tar := tar.NewReader(unzipped)
	for i := 0; i < archiveFileLimit; i++ {
		// Refresh lock every minute
		if time.Now().After(locked.Add(1 * time.Minute)) {
			err = lock.refresh()
			if err != nil {
				return nil, "", errors.WrapPrefix(err, "lost lock, aborting", 0)
			}
		}

		header, err := tar.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, "", errors.WrapPrefix(err, fmt.Sprintf("unable to process %s", repo), 0)
		}

		path := filepath.Join(dir, header.Name)
		info := header.FileInfo()
		if info.IsDir() {
			if err = os.MkdirAll(path, info.Mode()); err != nil {
				return nil, "", errors.WrapPrefix(err, fmt.Sprintf("unable to process %s", repo), 0)
			}
			continue
		}

		if !info.Mode().IsRegular() ||
			!strings.HasSuffix(header.Name, ".go") ||
			strings.Contains(header.Name, "vendor/") ||
			strings.Contains(header.Name, "testdata/") ||
			strings.HasSuffix(header.Name, "_test.go") {
			continue
		}

		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return nil, "", errors.WrapPrefix(err, fmt.Sprintf("unable to process %s", repo), 0)
		}
		defer file.Close()

		_, err = io.Copy(file, tar)
		if err != nil {
			return nil, "", errors.WrapPrefix(err, fmt.Sprintf("unable to process %s", repo), 0)
		}

		analyzer.Process(path)
		if err != nil {
			return nil, "", errors.WrapPrefix(err, fmt.Sprintf("unable to process %s", repo), 0)
		}

		os.Remove(path)
	}

	for i, issue := range analyzer.Issues {
		issue.File = strings.SplitN(strings.Replace(issue.File, dir, "", -1), "/", 3)[2]
		analyzer.Issues[i] = issue
	}

	logger.Printf("node %s done processing %s", nodeID, repo)
	return analyzer, res.Header.Get("ETag"), nil
}

func writeResults(resp http.ResponseWriter, t time.Time, path, tag, res string, missing bool) {
	if missing {
		resp.WriteHeader(http.StatusNotFound)
		return
	}

	if res == "" {
		resp.WriteHeader(http.StatusNotFound)
		return
	}

	results := map[string]interface{}{}
	err := json.Unmarshal([]byte(res), &results)
	if err != nil {
		logger.Printf("invalid JSON document at path %s", path)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	lifetime := t.Add(1*time.Hour).Sub(time.Now()) / time.Second
	if lifetime < 0 {
		lifetime = 0
	}

	resp.Header().Set("Content-Type", "application/json")
	resp.Header().Set("Cache-Control", fmt.Sprintf("max-age:%d", lifetime))
	raw, _ := json.Marshal(map[string]interface{}{
		"time":    t,
		"repo":    path,
		"tag":     strings.Trim(tag, `"`),
		"results": results,
	})
	resp.WriteHeader(http.StatusOK)
	resp.Write(raw)
}

func (w *worker) serveResults(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	user := vars["user"]
	repo := vars["repo"]

	path := fmt.Sprintf("github.com/%s/%s", user, repo)

	if !w.queueRequest(user, repo) {
		resp.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	t1, tag, res, missing, err := w.db.fetchResults(path)
	if err != nil && err != sql.ErrNoRows {
		logError(fmt.Sprintf("unable to fetch results for path %s", path), err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	if time.Now().Before(t1.Add(1*time.Hour)) && err == nil {
		writeResults(resp, t1, path, tag, res, missing)
		return
	}

	// Wait for at most 20 seconds for results to appear
	for i := 0; i < 20; i++ {
		t2, tag, res, missing, err := w.db.fetchResults(path)
		if err != nil && err != sql.ErrNoRows {
			logError(fmt.Sprintf("unable to fetch results for path %s", path), err)
			resp.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err == sql.ErrNoRows || t1.Equal(t2) {
			// No new results yet
			time.Sleep(1 * time.Second)
			continue
		}

		writeResults(resp, t2, path, tag, res, missing)
		return
	}

	resp.WriteHeader(http.StatusServiceUnavailable)
}
