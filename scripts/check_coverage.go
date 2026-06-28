package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run check_coverage.go coverage_output.txt [threshold]")
		os.Exit(1)
	}

	threshold := 100.0
	if len(os.Args) >= 3 {
		t, err := strconv.ParseFloat(os.Args[2], 64)
		if err == nil {
			threshold = t
		}
	}

	file, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	failed := false
	for scanner.Scan() {
		line := scanner.Text()
		// Example line: ok  	github.com/paleicikas/importinvoices/server/internal/worker	1.760s	coverage: 74.5% of statements
		// Or: ?   	github.com/paleicikas/importinvoices/server/internal/domain	[no test files]
		if !strings.Contains(line, "coverage:") {
			if strings.Contains(line, "[no test files]") {
				pkg := strings.Fields(line)[1]
				// We only care if it's not domain
				if !strings.HasSuffix(pkg, "/domain") && threshold >= 100.0 {
					fmt.Printf("Package %s has no test files, want %.1f%% coverage\n", pkg, threshold)
					failed = true
				}
			}
			continue
		}

		parts := strings.Fields(line)
		// parts: [ok, pkg, duration, coverage:, XX.X%, of, statements]
		if len(parts) < 5 {
			continue
		}

		pkg := parts[1]
		covStr := strings.TrimSuffix(parts[4], "%")
		
		if strings.HasSuffix(pkg, "/domain") {
			continue
		}

		cov, err := strconv.ParseFloat(covStr, 64)
		if err != nil {
			continue
		}

		if cov < threshold {
			fmt.Printf("Package %s has only %.1f%% coverage, want %.1f%%\n", pkg, cov, threshold)
			failed = true
		}
	}

	if failed {
		os.Exit(1)
	}
	if threshold >= 100.0 {
		fmt.Println("All packages have 100% coverage!")
	} else {
		fmt.Printf("All packages meet the %.1f%% coverage threshold!\n", threshold)
	}
}
