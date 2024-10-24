package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	SAMPLE_SIZE float32 = 0.10
	SAMPLE_MAX  int     = 16
	QUALITY     int     = 15
)

var (
	n_workers int = runtime.GOMAXPROCS(0)
)

type Pdf struct {
	Filename  string
	DirPath   string
	UsingOCR  bool
	Embedding Embeddings
}

func (p *Pdf) New(filename string) error {
	ok, err := check_file(filename)
	if !ok {
		return err
	}
	p.Filename = filename

	extension := filepath.Ext(filename)
	p.DirPath = filename[:len(filename)-len(extension)]

	p.Embedding.DataFile = p.DirPath + "/index.dat"

	err = os.Mkdir(p.DirPath, 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	embeddings_exist := true
	if _, err := os.Stat(p.Embedding.DataFile); os.IsNotExist(err) {
		embeddings_exist = false
	}

	if embeddings_exist {
		fmt.Println("Embeddings data exist")
		err := p.Embedding.Load()
		if err != nil {
			return err
		}
		fmt.Printf("Embeddings loaded: %d pages with %d dimensions each\n", p.Embedding.Shape[0], p.Embedding.Shape[1])
	} else {
		fmt.Println("Embeddings data and meta files do not exist")
		p.UsingOCR, err = p.UseOCR()
		if err != nil {
			return err
		}

		fmt.Println("Using OCR:", p.UsingOCR)
		err := p.CreateEmbeddings()
		if err != nil {
			return err
		}
	}

	return nil
}

func check_file(filename string) (bool, error) {
	if filename == "" {
		return false, fmt.Errorf("Filename is empty")
	}
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return false, fmt.Errorf("File does not exist")
	}
	return true, nil
}
