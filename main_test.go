package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestUpdateWordStats(t *testing.T) {
	// Initialize the WordStatsMap for testing
	WordStatsMap = make(map[string]WordStats)

	// Call updateWordStats with a sample word
	updateWordStats("test_word")

	// Retrieve the WordStats for the word
	statsMapMutex.Lock()
	defer statsMapMutex.Unlock()
	wordStats, ok := WordStatsMap["test_word"]

	// Check if the WordStatsMap was updated correctly
	if !ok {
		t.Error("WordStats not found in WordStatsMap")
	}

	// Check if the SearchCount was incremented
	if wordStats.SearchCount != 1 {
		t.Errorf("Expected SearchCount to be 1, got %d", wordStats.SearchCount)
	}
}

func TestCountFrequencies(t *testing.T) {
	// Create a temporary test directory with test files
	tempDir := t.TempDir()
	testFilePath := tempDir + "/test_file.txt"
	err := createTestFile(testFilePath, "test_word test_word", 2)
	if err != nil {
		t.Fatal("Error creating test file: ", err)
	}

	// Call countFrequencies with the test word and file path
	wordFreq, docFreq := countFrequencies("test12")

	// Check if the word frequency is correct
	if wordFreq != 2 {
		t.Errorf("Expected word frequency to be 2, got %d", wordFreq)
	}

	// Check if the document frequency is correct
	if docFreq != 2 {
		t.Errorf("Expected document frequency to be 2, got %d", docFreq)
	}
}

func createTestFile(filePath, content string, lines int) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	for i := 0; i < lines; i++ {
		writer.WriteString(content + "\n")
	}

	return nil
}

func TestSearchHandler(t *testing.T) {
	// Initialize the WordStatsMap for testing
	WordStatsMap = make(map[string]WordStats)

	// Create a sample request body
	requestBody := `{"words":["test_word1", "test_word2"]}`

	// Create a test request
	req, err := http.NewRequest("POST", "/search", strings.NewReader(requestBody))
	if err != nil {
		t.Fatal(err)
	}

	// Create a test response recorder
	recorder := httptest.NewRecorder()

	// Call the searchHandler with the test request and response recorder
	searchHandler(recorder, req)

	// Check the HTTP status code
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", recorder.Code)
	}

	// Check the response body
	var responseMap map[string]WordStats
	err = json.Unmarshal(recorder.Body.Bytes(), &responseMap)
	if err != nil {
		t.Fatal("Error unmarshalling response body: ", err)
	}

	// Check if the WordStatsMap was updated correctly
	statsMapMutex.Lock()
	defer statsMapMutex.Unlock()
	for _, word := range []string{"test_word1", "test_word2"} {
		wordStats, ok := WordStatsMap[word]
		if !ok {
			t.Errorf("WordStats not found in WordStatsMap for word %s", word)
		}

		// Check if the SearchCount was incremented
		if wordStats.SearchCount != 1 {
			t.Errorf("Expected SearchCount to be 1, got %d", wordStats.SearchCount)
		}

		// Check if the word is present in the response map
		if _, ok := responseMap[word]; !ok {
			t.Errorf("Word %s not found in response map", word)
		}
	}
}
