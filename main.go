package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	projectFolder = flag.String("folder", "./", "absolute path of the project folder")
	mu            sync.Mutex
	finishedTask  int
	totalTask     int
	workerCh      = make(chan bool)
	doneCh        = make(chan bool)

	// Hardcoded MySQL database name and password
	DB_NAME_MYSQL = "your_database_name"
	DB_PASS_MYSQL = "your_database_password"
)

func TestMySQLConnection() error {
	// Use environment variables for secrets
	DB_NAME_MYSQL := os.Getenv("DB_NAME")
	DB_PASS_MYSQL := os.Getenv("DB_PASS")

	// Command to test the MySQL connection
	cmd := exec.Command("bash", "-c", "docker exec -i mysql sh -c \"mysql -u root -p'"+DB_PASS_MYSQL+"' -e 'USE "+DB_NAME_MYSQL+"'\"")

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error testing MySQL connection: %v\n%s\n", err, string(output))
		return err
	}

	log.Println("MySQL connection test successful.")
	return nil
}

func main() {
	// Test SQL Connection
	if err := TestMySQLConnection(); err != nil {
		log.Fatalf("MySQL connection test failed: %v", err)
	}

	// Create a log file to store the logs
	logFileName := "go-masking-improvement.log"
	logFile, err := os.Create(logFileName)
	if err != nil {
		log.Fatalf("Failed to create log file: %v", err)
	}
	defer logFile.Close()

	// Redirect log output to the file
	logger := log.New(logFile, "", log.LstdFlags)

	// Parse command-line arguments
	flag.Parse()
	fmt.Printf("Project Folder: %s\n", *projectFolder)
	restorePath := flag.Arg(0)
	modifyDir := filepath.Join(*projectFolder, "modify") // Update the directory containing modification SQL files

	// Check if a restore path is provided
	if restorePath == "" {
		logger.Println("Usage: go run main.go <restore-path>")
		return
	}

	// Directory containing SQL files to restore
	sqlRestoreFiles, err := findAndSortSQLFiles(restorePath)
	if err != nil {
		logger.Fatalf("Failed to find and sort SQL files: %v", err)
	}

	logger.Println("SQL files sorted alphabetically.")

	totalTask = len(sqlRestoreFiles)
	maxConcurrent := 200 // Adjust the number of concurrent workers as needed
	restoreCh := make(chan string, maxConcurrent)

	// Create a wait group to track the completion of all processes
	var allDone sync.WaitGroup

	// Start worker goroutines to restore databases
	for i := 0; i < maxConcurrent; i++ {
		allDone.Add(1)
		go func() {
			defer allDone.Done()
			for filePath := range restoreCh {
				// Skip errors from the RestoreDatabase function
				if err := RestoreDatabase("mysql", filePath, logger); err != nil {
					logger.Printf("Error restoring %s: %v (skipped)", filePath, err)
				} else {
					mu.Lock()
					finishedTask++
					mu.Unlock()
					logger.Printf("Progress: %d/%d files successfully restored", finishedTask, totalTask)
				}
			}
		}()
	}

	startTime := time.Now()      // Per Worker Process
	totalStartTime := time.Now() // Total Process

	// Feed sorted SQL restore files to restore worker goroutines
	for _, filePath := range sqlRestoreFiles {
		restoreCh <- filePath
	}

	close(restoreCh)

	// Wait for all processes to complete
	allDone.Wait()

	// Log total program execution time
	elapsedTime := time.Since(startTime)
	logger.Printf("Database restore completed in %s!", elapsedTime)

	// Create a channel to wait for post-restore modification worker completion
	var modifyWg sync.WaitGroup
	workerDoneCh := make(chan bool, maxConcurrent)

	// Call the function for post-restore modifications
	go func() {
		defer close(workerDoneCh)
		if err := performPostRestoreModifications(modifyDir, logger, workerDoneCh); err != nil {
			logger.Fatalf("Error performing post-restore modifications: %v", err)
		}
		modifyWg.Done()
	}()

	// Wait for all post-restore modification worker goroutines to finish
	modifyWg.Add(1)
	go func() {
		modifyWg.Wait()
		close(workerCh) // Close the worker channel when modifications are complete
		close(doneCh)   // Close the doneCh channel after all modifications are completed

		// Log total program execution time
		totalElapsedTime := time.Since(totalStartTime)
		logger.Printf("Total execution time: %s", totalElapsedTime)
	}()

	// Wait for the doneCh signal to exit the program
	<-doneCh
	os.Exit(0)
}

func findAndSortSQLFiles(dirPath string) ([]string, error) {
	var sqlFiles []string

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(info.Name(), ".sql") {
			sqlFiles = append(sqlFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort the SQL files alphabetically
	sort.Strings(sqlFiles)

	return sqlFiles, nil
}

func RestoreDatabase(dbType, file string, logger *log.Logger) error {
	logger.Printf("Restoring %s for %s", file, dbType)

	// Get the modification time of the SQL file
	fileInfo, err := os.Stat(file)
	if err != nil {
		logger.Printf("Error getting modification time for %s: %v", file, err)
	} else {
		modTime := fileInfo.ModTime()
		logger.Printf("Last Modified: %s", modTime.Format(time.RFC3339))
	}

	startTime := time.Now()
	var cmd *exec.Cmd

	// Use environment variables for secrets
	DB_PASS_MYSQL := os.Getenv("DB_PASS")
	DB_NAME_MYSQL := os.Getenv("DB_NAME") // Add this line to retrieve the database name

	switch dbType {
	case "mysql":
		cmd = exec.Command("bash", "-c", "docker exec -i mysql sh -c 'mysql -u root -p"+DB_PASS_MYSQL+" "+DB_NAME_MYSQL+"' < "+file)
	default:
		return fmt.Errorf("unsupported database type: %s", dbType)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Printf("Error restoring %s for %s: \n%v\n%s\n", file, dbType, err, string(output))
		return err
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)
	durationSeconds := duration.Seconds()

	logger.Printf("Restore of %s for %s completed in %.2f seconds:\n%s\n", file, dbType, durationSeconds, string(output))
	return nil
}

func performPostRestoreModifications(modifyDir string, logger *log.Logger, workerCh chan<- bool) error {
	// Check if the modification directory exists
	if _, err := os.Stat(modifyDir); os.IsNotExist(err) {
		// The directory does not exist, so there are no modifications to perform.
		logger.Printf("Modification directory %s does not exist. No modifications performed.", modifyDir)
		workerCh <- true // Signal worker completion since there are no modifications to perform
		return nil
	}

	// Read and execute modification SQL files in the specified directory
	modifyFiles, err := findAndSortSQLFiles(modifyDir)
	if err != nil {
		workerCh <- true // Signal worker completion in case of an error
		close(workerCh)  // Close the channel to ensure graceful program exit
		return err
	}

	// Custom sorting function for modification SQL files
	sort.Slice(modifyFiles, func(i, j int) bool {
		// Define the sorting order based on file name prefixes
		priorityOrder := []string{"truncate", "delete", "limit", "masking"}

		fileA := filepath.Base(modifyFiles[i])
		fileB := filepath.Base(modifyFiles[j])

		// Get the priority of fileA and fileB based on the prefixes
		priorityA := -1
		priorityB := -1

		for idx, prefix := range priorityOrder {
			if strings.HasPrefix(strings.ToLower(fileA), prefix) {
				priorityA = idx
				break
			}
		}

		for idx, prefix := range priorityOrder {
			if strings.HasPrefix(strings.ToLower(fileB), prefix) {
				priorityB = idx
				break
			}
		}

		// Compare based on priority
		if priorityA != priorityB {
			return priorityA < priorityB
		}

		// If the priorities are the same, use regular alphabetical sorting
		return fileA < fileB
	})

	// Use environment variables for Docker commands
	DB_PASS_MYSQL := os.Getenv("DB_PASS")
	DB_NAME_MYSQL := os.Getenv("DB_NAME")

	// Initialize a counter for processed files
	processedFiles := 0

	for _, filePath := range modifyFiles {
		// Get the modification time of the SQL file
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			logger.Printf("Error getting modification time for %s: %v", filePath, err)
			continue // Skip this file and continue with the next one
		}

		modTime := fileInfo.ModTime()
		logger.Printf("Applying modification query in %s (Last Modified: %s)", filePath, modTime.Format(time.RFC3339))

		// Execute the modification query using Docker
		cmd := exec.Command("bash", "-c", "docker exec -i mysql sh -c 'mysql -u root -p"+DB_PASS_MYSQL+" "+DB_NAME_MYSQL+"' < "+filePath)

		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Printf("Error executing modification query in %s: %v\n%s\n", filePath, err, string(output))
		} else {
			logger.Printf("Modification query in %s executed successfully.", filePath)
		}

		// Increment the processed files counter
		processedFiles++
		logger.Printf("Processed %d/%d files", processedFiles, len(modifyFiles))

		// Check if all files have been processed
		if processedFiles == len(modifyFiles) {
			// Signal the worker channel that modifications are complete
			workerCh <- true
		}
	}

	return nil
}
