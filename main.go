// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"log"
	"net/http"
	"text/template"
	"os"
	"io"
	"runtime"
	"path"
)

var addr = flag.String("addr", ":8080", "http service address")
var homeTemplate = template.Must(template.ParseFiles("home.html"))

const UPLOAD_PATH = "/var/www/html/golang/"

func serveHome(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method)

	if r.URL.Path == "/" && r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		homeTemplate.Execute(w, r.Host)
	} else if r.URL.Path == "/ajax" && r.Method == "POST" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		homeTemplate.Execute(w, r.Host)
		uploader(w, r)
	} else {
		http.Error(w, "Not found", 404)
		return
	}
}

func uploader(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(100000)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	m := r.MultipartForm
	files := m.File["file"]
	file, err := files[0].Open()
	defer file.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dst, err := os.Create(UPLOAD_PATH + files[0].Filename)
	defer dst.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func main() {
	flag.Parse()
	hub := newHub()
	go hub.run()
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func getCurrentDir() string {
	_, filename, _, _ := runtime.Caller(1)

	return path.Dir(filename)
}
