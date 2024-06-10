package main

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type WordStats struct {
	TF            uint // Text Frequency
	DF            uint // Document Frequency
	SearchCount   uint // Number of times users searched for the word
	LastTF        uint // Last Text Frequency
	LastDF        uint // Last Document Frequency
	SearchHistory []struct {
		TF uint // Text Frequency for the search operations made on this word
		DF uint // Document Frequency for the search operations requested on this word
	}
}

const (
	// workerPoolSize = 10
	filePattern = "./data/*.txt"
	// delimiter      = "," // Change this to adjust the delimiter character
	// searchTimeout = 5 * time.Second
)

var (
	// workerPool   = make(chan struct{}, workerPoolSize)
	// fileMutex    = &sync.Mutex{}
	statsMapMutex = &sync.Mutex{} // for synchronization and avoiding mutual exclusivity, reading & writing to map
	fileMutex     = &sync.Mutex{} // for synchronization and avoiding mutual exclusivity, reading & writing to files
	WordStatsMap  = make(map[string]WordStats)
)

// TODO:: use channels & contexts for more efficient handling
// TODO:: use in-memory db, then persist in files to avoid crashing and handle large load

// TODO:: consider reading files in chunks for better performance when files go wildly large instead of scanning

// TODO:: consider creating a separate pckg for stats and even one more can be for files reading operations,
// delegate the work load and keep main for server handling requests

// TODO:: refactor for separation of concerns so files delimiters can be changes easily with little to no change in impl
// delimiters can be comma, space, newlines, or any

func searchHandler(w http.ResponseWriter, r *http.Request) {
	var words []string

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Couldn't read request body", http.StatusBadRequest)
		log.Printf("Couldn't read request body: %v", err)
		return
	}

	// Unmarshal the JSON body
	var requestBody map[string][]string
	err = json.Unmarshal(body, &requestBody)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		log.Printf("Invalid request body: %v", err)
		return
	}

	// Access the "words" array and process it
	words, ok := requestBody["words"]
	if !ok {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		log.Printf("Invalid request body: %v", err)
		return
	}

	if len(words) == 0 {
		http.Error(w, "Missing 'words' to search for", http.StatusBadRequest)
		log.Printf("Missing 'words' to search for")
		return
	}

	var wg sync.WaitGroup

	for _, word := range words {
		wg.Add(1)
		go func(word string) {
			defer wg.Done()
			updateWordStats(word)
		}(word)
	}

	wg.Wait()

	// return only requested words
	res := make(map[string]WordStats)
	for _, word := range words {
		res[word] = WordStatsMap[word]
	}

	// Return the statistics as part of the response
	statsJSON, err := json.Marshal(res)
	if err != nil {
		log.Printf("Error marshalling data: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(statsJSON)
}

func updateWordStats(word string) {
	statsMapMutex.Lock()
	defer statsMapMutex.Unlock()

	info, ok := WordStatsMap[word]
	if !ok {
		info = WordStats{}
		WordStatsMap[word] = info
	}

	info.SearchCount++

	countWordFreq, countDocFreq := countFrequencies(word)
	info.LastTF, info.LastDF = info.TF, info.DF
	info.TF, info.DF = countWordFreq, countDocFreq

	info.SearchHistory = append(info.SearchHistory, struct {
		TF uint
		DF uint
	}{countWordFreq, countDocFreq})
	WordStatsMap[word] = info

}

func countFrequencies(word string) (uint, uint) {

	// TODO:: use pointer instead to pass around more efficiently
	var countWordFreq uint
	var countDocFreq uint

	filePaths, err := getAllFilesPaths()
	if err != nil {
		log.Fatal("Stopping Execution due to following error: ", err)
		panic(err)
	}

	var wg sync.WaitGroup

	// TODO:: Refactor to extract into a separate func `processFiles(filePaths []string)([]string, error)` e.g
	for _, filePath := range filePaths {
		wg.Add(1)
		// TODO:: in the newly created func for file processing, extract this and just spin routines to call the new func
		go func(filePath, word string) {
			defer wg.Done()
			fileMutex.Lock()
			defer fileMutex.Unlock()
			reader, err := os.Open(filePath)
			if err != nil {
				log.Printf("Error opening file %s for word %v: %v\n", filePath, word, err)
				return
			}
			defer reader.Close()

			scanner := bufio.NewScanner(reader)

			scanner.Split(bufio.ScanWords)

			found := false
			for scanner.Scan() {
				if scanner.Text() == word {
					countWordFreq++
					if !found {
						found = true
						countDocFreq++
					}
				}
			}
			if err := scanner.Err(); err != nil {
				log.Printf("Error scanning file %s for word %v: %v\n", filePath, word, err)
			}
		}(filePath, word)
	}
	wg.Wait()

	return countWordFreq, countDocFreq
}

func getAllFilesPaths() ([]string, error) {
	filePaths, err := filepath.Glob(filePattern)
	if err != nil {
		log.Fatal("Error finding data files: ", err)
		return nil, err
	}
	return filePaths, nil
}

func main() {
	http.HandleFunc("/search", searchHandler)
	log.Fatal(http.ListenAndServe(":4200", nil))
}
