package main

import (
	"sync"
)

// ExecuteParallel executes a function for each item in parallel with configurable concurrency
// T is the input type, R is the result type
func ExecuteParallel[T any, R any](items []T, fn func(T) (R, error), parallel bool, maxConcurrent int) []R {
	results := make([]R, len(items))
	
	if !parallel || maxConcurrent <= 1 || len(items) <= 1 {
		// Sequential execution
		for i, item := range items {
			result, _ := fn(item)
			results[i] = result
		}
		return results
	}

	// Parallel execution with semaphore for concurrency control
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, item := range items {
		wg.Add(1)
		go func(idx int, item T) {
			defer wg.Done()
			
			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()
			
			// Execute function
			result, _ := fn(item)
			
			// Store result safely
			mu.Lock()
			results[idx] = result
			mu.Unlock()
		}(i, item)
	}

	wg.Wait()
	return results
}

// ParallelResult wraps a result with its index and error
type ParallelResult[T any] struct {
	Index  int
	Result T
	Error  error
}

// ExecuteParallelWithErrors executes a function for each item in parallel and returns results with errors
func ExecuteParallelWithErrors[T any, R any](items []T, fn func(T) (R, error), parallel bool, maxConcurrent int) []ParallelResult[R] {
	results := make([]ParallelResult[R], len(items))
	
	if !parallel || maxConcurrent <= 1 || len(items) <= 1 {
		// Sequential execution
		for i, item := range items {
			result, err := fn(item)
			results[i] = ParallelResult[R]{
				Index:  i,
				Result: result,
				Error:  err,
			}
		}
		return results
	}

	// Parallel execution with semaphore for concurrency control
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, item := range items {
		wg.Add(1)
		go func(idx int, item T) {
			defer wg.Done()
			
			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()
			
			// Execute function
			result, err := fn(item)
			
			// Store result safely
			mu.Lock()
			results[idx] = ParallelResult[R]{
				Index:  idx,
				Result: result,
				Error:  err,
			}
			mu.Unlock()
		}(i, item)
	}

	wg.Wait()
	return results
}