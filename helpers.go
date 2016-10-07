package main

import (
	"github.com/go-errors/errors"
)

func logError(msg string, err interface{}) {
	if err != nil {
		if _, ok := err.(*errors.Error); ok {
			logger.Printf("%s: %s", msg, err.(*errors.Error).ErrorStack())
		} else {
			logger.Printf("%s: %v", msg, err)
		}
	}
}
