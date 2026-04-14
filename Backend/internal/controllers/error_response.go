package controllers

import (
	"log"
	"net/http"
	"path/filepath"
	"runtime"
)

const internalServerErrorMessage = "内部エラーが発生しました"

func writeInternalServerError(w http.ResponseWriter, err error) {
	if err != nil {
		if _, file, line, ok := runtime.Caller(1); ok {
			log.Printf("[ERROR] %s:%d %v", filepath.Base(file), line, err)
		} else {
			log.Printf("[ERROR] %v", err)
		}
	}
	http.Error(w, internalServerErrorMessage, http.StatusInternalServerError)
}

func writeErrorByStatus(w http.ResponseWriter, status int, err error) {
	if status >= http.StatusInternalServerError {
		writeInternalServerError(w, err)
		return
	}
	if err != nil {
		writeErrorByStatus(w, status, err)
		return
	}
	http.Error(w, http.StatusText(status), status)
}
