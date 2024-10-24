package main

import (
	"fmt"
)

var (
	books []string = []string{
		"books/Concurrency in Go.pdf",
		"books/linear-guest.pdf",
	}
)

func main() {
	pdf := Pdf{}
	err := pdf.New(books[0])
	if err != nil {
		fmt.Println(err)
		return
	}
}
