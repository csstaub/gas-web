package main

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/go-errors/errors"
	_ "github.com/go-sql-driver/mysql"
)

type lock interface {
	unlock() error
	refresh() error
}

type database interface {
	// Lock operations
	lockPath(node, path string, lifetime time.Duration) (lock, error)
	isLocked(path string) (bool, error)

	// Results storage
	storeResults(path, results string) error
	fetchResults(path string) (time.Time, string, error)
}

type sqlDatabase struct {
	*sql.DB
}

type sqlLock struct {
	db       *sqlDatabase
	lifetime time.Duration
	node     string
	path     string
	hash     []byte
}

func newSQLDatabase() (database, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return nil, errors.New("missing DATABASE_URL environment variable")
	}

	db, err := sql.Open("mysql", url)
	if err != nil {
		return nil, errors.WrapPrefix(err, "unable to talk to DB", 0)
	}

	return &sqlDatabase{db}, nil
}

func (db *sqlDatabase) lockPath(node, path string, lifetime time.Duration) (lock, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, errors.New(err)
	}

	logger.Printf("node %s requesting lock %s for %s", node, path, lifetime.String())

	hash := sha256.Sum256([]byte(path))
	row := tx.QueryRow("SELECT holder, timestamp, lifetime FROM locks WHERE hash = ?", hash[:])

	var lockHolder string
	var lockTimestamp int64
	var lockLifetime int64
	err = row.Scan(&lockHolder, &lockTimestamp, &lockLifetime)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.WrapPrefix(err, "error talking to database", 0)
	}

	if err != nil {
		// Insert lock
		_, err := tx.Exec(
			"INSERT INTO locks (hash, description, holder, timestamp, lifetime) VALUES (?, ?, ?, ?, ?)",
			hash[:], path, node, time.Now().Unix(), lifetime/time.Second)
		if err != nil {
			logError(fmt.Sprintf("node %s error on rollback", node), tx.Rollback())
			logger.Printf("node %s unable to acquire lock %s (DB error)", node, path)
			return nil, errors.WrapPrefix(err, "unable to acquire lock (insert failed)", 0)
		}
	} else {
		expiry := time.Unix(lockTimestamp, 0).Add(time.Duration(lockLifetime) * time.Second)

		if lockHolder == node || time.Now().After(expiry) {
			_, err := tx.Exec(
				"UPDATE locks SET holder = ?, timestamp = ?, lifetime = ? WHERE hash = ?",
				node, time.Now().Unix(), lifetime/time.Second, hash[:])
			if err != nil {
				logError(fmt.Sprintf("node %s error on rollback", node), tx.Rollback())
				logger.Printf("node %s unable to acquire lock %s (DB error)", node, path)
				return nil, errors.WrapPrefix(err, "unable to acquire lock (update failed)", 0)
			}
		} else {
			logger.Printf("node %s unable to acquire lock %s (already locked)", node, path)
			return nil, nil
		}
	}

	err = tx.Commit()
	if err != nil {
		logger.Printf("node %s unable to acquire lock %s (commit failed)", node, path)
		return nil, nil
	}
	logger.Printf("node %s acquired lock %s for %s", node, path, lifetime.String())
	return &sqlLock{db, lifetime, node, path, hash[:]}, nil
}

func (db *sqlDatabase) isLocked(path string) (bool, error) {
	hash := sha256.Sum256([]byte(path))
	row := db.QueryRow("SELECT timestamp, lifetime FROM locks WHERE hash = ?", hash[:])

	var lockTimestamp int64
	var lockLifetime int64
	err := row.Scan(&lockTimestamp, &lockLifetime)
	if err != nil && err != sql.ErrNoRows {
		return false, errors.WrapPrefix(err, "unable to check lock", 0)
	}

	if err == sql.ErrNoRows {
		return false, nil
	}

	expiry := time.Unix(lockTimestamp, 0).Add(time.Duration(lockLifetime) * time.Second)
	if time.Now().After(expiry) {
		return false, nil
	}

	return true, nil
}

func (sl *sqlLock) refresh() error {
	logger.Printf("node %s refreshing lock %s for %s", sl.node, sl.path, sl.lifetime.String())

	r, err := sl.db.Exec(
		"UPDATE locks SET timestamp = ?, lifetime = ? WHERE hash = ?, holder = ?",
		time.Now().Unix(), sl.lifetime/time.Second, sl.hash, sl.node)
	if err != nil {
		logger.Printf("node %s unable to refresh lock %s (DB error)", sl.node, sl.path)
		return errors.WrapPrefix(err, "unable to refresh lock (update failed)", 0)
	}

	n, err := r.RowsAffected()
	if err != nil {
		logger.Printf("node %s unable to refresh lock %s (result error)", sl.node, sl.path)
		return errors.WrapPrefix(err, "unable to refresh (update failed)", 0)
	}
	if n != 1 {
		logger.Printf("node %s unable to refresh lock %s (lost lock)", sl.node, sl.path)
		return errors.New("unable to refresh lock: lost lock?")
	}
	return nil
}

func (sl *sqlLock) unlock() error {
	logger.Printf("node %s dropping lock %s", sl.node, sl.path)

	_, err := sl.db.Exec("DELETE FROM locks WHERE hash = ? AND holder = ?", sl.hash, sl.node)
	if err != nil {
		return errors.WrapPrefix(err, "unable to drop lock", 0)
	}
	return nil
}

func (db *sqlDatabase) storeResults(path, results string) error {
	hash := sha256.Sum256([]byte(path))
	_, err := db.Exec(
		`INSERT INTO results (hash, timestamp, results) VALUES (?, ?, ?) 
		 ON DUPLICATE KEY UPDATE timestamp = ?, results = ?`,
		hash[:], time.Now().Unix(), results, time.Now().Unix(), results)
	if err != nil {
		return errors.New(err)
	}
	return nil
}

func (db *sqlDatabase) fetchResults(path string) (time.Time, string, error) {
	hash := sha256.Sum256([]byte(path))
	r := db.QueryRow("SELECT timestamp, results FROM results WHERE hash = ?", hash[:])

	var timestamp int64
	var results string
	err := r.Scan(&timestamp, &results)
	if err != nil {
		return time.Now(), "", errors.New(err)
	}

	return time.Unix(timestamp, 0), results, nil
}
