package main

import "net/http"

const sharedDir = "/home/trueabc/books"

// const sharedDir = "/"

func main() {
	http.Handle("/staticfile/", http.StripPrefix("/staticfile/", http.FileServer(http.Dir(sharedDir))))

	//Listen on port 8080
	http.ListenAndServe(":8080", nil)
}
