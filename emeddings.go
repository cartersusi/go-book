package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"log"
	"os"
	"sync"
	"unsafe"

	"github.com/gen2brain/go-fitz"
	"github.com/joho/godotenv"
	"github.com/otiai10/gosseract/v2"
	"github.com/sashabaranov/go-openai"
	"golang.org/x/sys/unix"
)

type Embeddings struct {
	DataFile string
	Data     [][]float32
	Shape    [2]int32
}

func (p *Pdf) CreateWithOCR() error {
	fitz_doc, err := fitz.New(p.Filename)
	if err != nil {
		return err
	}

	n_pages := fitz_doc.NumPage()

	// Maybe move into worker func, needs testing
	imgs := make([]*image.RGBA, n_pages)
	for i := 0; i < n_pages; i++ {
		imgs[i], err = fitz_doc.Image(i)
		if err != nil {
			return err
		}
	}
	go fitz_doc.Close()

	n_batches := n_pages / n_workers
	//leftover := n_pages % n_workers

	jobs := make(chan int, n_pages)
	results := make(chan error, n_pages)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for w := 0; w < n_workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			ocr_client := gosseract.NewClient()
			openai_client := openai.NewClient(openai_key())

			if err != nil {
				results <- fmt.Errorf("OCR error on worker %d: %v", workerID, err)
				return
			}

			for pageIdx := range jobs {
				text, err := Img2Text(ocr_client, imgs[pageIdx])
				if err != nil {
					results <- fmt.Errorf("OCR error on page %d: %v", pageIdx, err)
					continue
				}

				embedding, err := EmbeddingsFromText(openai_client, text)
				if err != nil {
					results <- fmt.Errorf("embedding error on page %d: %v", pageIdx, err)
					continue
				}

				mu.Lock()
				p.Embedding.Data[pageIdx] = embedding
				mu.Unlock()

				go ocr_client.Close()
				results <- nil
			}
		}(w)
	}

	go func() {
		for i := 0; i < n_batches*n_workers; i++ {
			jobs <- i
		}
		for i := n_batches * n_workers; i < n_pages; i++ {
			jobs <- i
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	for i := 0; i < n_pages; i++ {
		if err := <-results; err != nil {
			return err
		}
	}

	p.Embedding.Save()
	return nil
}

func (p *Pdf) CreateEmbeddings() error {
	if p.UsingOCR {
		fmt.Println("Using OCR")
		return p.CreateWithOCR()
	}
	fmt.Println("Not using OCR")

	fitz_doc, err := fitz.New(p.Filename)
	if err != nil {
		return err
	}
	fmt.Println("Fitz doc created")

	n_pages := fitz_doc.NumPage()
	p.Embedding.Data = make([][]float32, n_pages)
	fmt.Println("Number of pages:", n_pages)

	// Maybe move into worker func, needs testing
	fmt.Println("Extracting text from pages")
	pages := make([]string, n_pages)
	for i := 0; i < n_pages; i++ {
		pages[i], err = fitz_doc.Text(i)
		if err != nil {
			return err
		}
	}
	fmt.Println("Text extracted from pages")
	go fitz_doc.Close()
	fmt.Println("Fitz doc closed")

	n_batches := n_pages / n_workers
	fmt.Println("Number of batches:", n_batches)

	jobs := make(chan int, n_pages)
	results := make(chan error, n_pages)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for w := 0; w < n_workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			fmt.Println("Worker", workerID)
			openai_client := openai.NewClient(openai_key())
			if err != nil {
				results <- fmt.Errorf("OCR error on worker %d: %v", workerID, err)
				return
			}

			for pageIdx := range jobs {
				fmt.Println("Page", pageIdx)
				embedding, err := EmbeddingsFromText(openai_client, pages[pageIdx])
				if err != nil {
					results <- fmt.Errorf("embedding error on page %d: %v", pageIdx, err)
					continue
				}

				mu.Lock()
				p.Embedding.Data[pageIdx] = embedding
				mu.Unlock()

				results <- nil
			}
		}(w)
	}

	go func() {
		for i := 0; i < n_batches*n_workers; i++ {
			jobs <- i
		}
		for i := n_batches * n_workers; i < n_pages; i++ {
			jobs <- i
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	for i := 0; i < n_pages; i++ {
		if err := <-results; err != nil {
			return err
		}
	}

	if p.Embedding.DataFile == "" {
		return fmt.Errorf("DataFile or MetaFile is empty")
	}

	if p.Embedding.Data == nil || len(p.Embedding.Data) == 0 {
		return fmt.Errorf("Embedding data is empty")
	}

	err = p.Embedding.Save()
	if err != nil {
		return err
	}

	return nil
}

func EmbeddingsFromText(openai_client *openai.Client, text string) ([]float32, error) {
	resp, err := openai_client.CreateEmbeddings(
		context.Background(),
		openai.EmbeddingRequest{
			Input: text,
			Model: openai.SmallEmbedding3,
		},
	)
	if err != nil {
		return nil, err
	}

	return resp.Data[0].Embedding, nil
}

func (e *Embeddings) Save() error {
	if e.DataFile == "" {
		return fmt.Errorf("DataFile is empty")
	}

	f, err := os.Create(e.DataFile)
	if err != nil {
		return err
	}
	defer f.Close()

	numRows := int32(len(e.Data))
	if numRows == 0 {
		return fmt.Errorf("empty embedding data")
	}
	numCols := int32(len(e.Data[0]))

	err = binary.Write(f, binary.LittleEndian, numRows)
	if err != nil {
		return err
	}
	err = binary.Write(f, binary.LittleEndian, numCols)
	if err != nil {
		return err
	}

	flatData := make([]float32, 0, int(numRows)*int(numCols))
	for _, row := range e.Data {
		flatData = append(flatData, row...)
	}

	err = binary.Write(f, binary.LittleEndian, flatData)
	if err != nil {
		return err
	}

	return nil
}

func (e *Embeddings) Load() error {
	if e.DataFile == "" {
		return fmt.Errorf("DataFile is empty")
	}

	f, err := os.OpenFile(e.DataFile, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	mmapData, err := unix.Mmap(int(f.Fd()), 0, int(fi.Size()),
		unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		return err
	}
	defer unix.Munmap(mmapData)

	e.Shape[0] = int32(binary.LittleEndian.Uint32(mmapData[0:4]))
	e.Shape[1] = int32(binary.LittleEndian.Uint32(mmapData[4:8]))

	dataOffset := 8
	flatData := unsafe.Slice(
		(*float32)(unsafe.Pointer(&mmapData[dataOffset])),
		(len(mmapData)-dataOffset)/4,
	)

	totalSize := int(e.Shape[0]) * int(e.Shape[1])
	dataCopy := make([]float32, totalSize)
	copy(dataCopy, flatData[:totalSize])

	e.Data = make([][]float32, int(e.Shape[0]))
	for i := range e.Data {
		start := i * int(e.Shape[1])
		end := start + int(e.Shape[1])
		e.Data[i] = make([]float32, int(e.Shape[1]))
		copy(e.Data[i], dataCopy[start:end])
	}

	return nil
}

func openai_key() string {
	var key string
	key = os.Getenv("OPENAI_KEY")
	if key != "" {
		return key
	}

	err := godotenv.Load()
	if err != nil {
		return ""
	}

	key = os.Getenv("OPENAI_KEY")
	if key == "" {
		log.Fatal("OPENAI_KEY not found")
	}

	return key
}
