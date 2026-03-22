package logic

import (
	"context"
	"fmt"
	"sync"
)

// Task represents a unit of work that can be executed
type Task struct {
	ID           string                 // Unique task ID
	Dependencies []string               // IDs of tasks this task depends on
	Data         interface{}            // Task-specific data
	Execute      func(ctx context.Context, data interface{}) error // Execution function
}

// TaskResult represents the result of a task execution
type TaskResult struct {
	TaskID string
	Error  error
}

// DependencyGraph represents a directed acyclic graph of tasks
type DependencyGraph struct {
	tasks       map[string]*Task          // All tasks by ID
	dependents  map[string][]string       // Map of task ID -> tasks that depend on it
	inDegree    map[string]int            // Number of unresolved dependencies for each task
	mu          sync.RWMutex
}

// NewDependencyGraph creates a new empty dependency graph
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		tasks:      make(map[string]*Task),
		dependents: make(map[string][]string),
		inDegree:   make(map[string]int),
	}
}

// AddTask adds a task to the graph
func (g *DependencyGraph) AddTask(task *Task) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.tasks[task.ID]; exists {
		return fmt.Errorf("task %s already exists", task.ID)
	}

	g.tasks[task.ID] = task
	g.inDegree[task.ID] = len(task.Dependencies)

	// Build reverse dependency map (dependents)
	for _, depID := range task.Dependencies {
		g.dependents[depID] = append(g.dependents[depID], task.ID)
	}

	return nil
}

// GetReadyTasks returns tasks that have no unresolved dependencies
func (g *DependencyGraph) GetReadyTasks() []*Task {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var ready []*Task
	for id, degree := range g.inDegree {
		if degree == 0 {
			if task, exists := g.tasks[id]; exists {
				ready = append(ready, task)
			}
		}
	}
	return ready
}

// MarkCompleted marks a task as completed and updates dependent tasks
func (g *DependencyGraph) MarkCompleted(taskID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Decrease in-degree for all dependent tasks
	for _, dependentID := range g.dependents[taskID] {
		g.inDegree[dependentID]--
	}

	// Remove task from graph
	delete(g.tasks, taskID)
	delete(g.inDegree, taskID)
	delete(g.dependents, taskID)
}

// HasTasks returns true if there are still tasks pending
func (g *DependencyGraph) HasTasks() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.tasks) > 0
}

// Executor handles concurrent execution of dependent tasks
type Executor struct {
	maxConcurrency int
	graph          *DependencyGraph
	results        chan TaskResult
	wg             sync.WaitGroup
}

// NewExecutor creates a new executor with specified concurrency limit
func NewExecutor(maxConcurrency int) *Executor {
	if maxConcurrency <= 0 {
		maxConcurrency = 3 // Default concurrency
	}
	return &Executor{
		maxConcurrency: maxConcurrency,
		graph:          NewDependencyGraph(),
		results:        make(chan TaskResult, 100),
	}
}

// AddTask adds a task to the executor
func (e *Executor) AddTask(task *Task) error {
	return e.graph.AddTask(task)
}

// Execute runs all tasks respecting dependencies
func (e *Executor) Execute(ctx context.Context) []TaskResult {
	var allResults []TaskResult
	semaphore := make(chan struct{}, e.maxConcurrency)

	for e.graph.HasTasks() {
		readyTasks := e.graph.GetReadyTasks()
		if len(readyTasks) == 0 && e.graph.HasTasks() {
			// Deadlock detected - circular dependency
			allResults = append(allResults, TaskResult{
				TaskID: "",
				Error:  fmt.Errorf("circular dependency detected"),
			})
			break
		}

		// Execute all ready tasks concurrently
		for _, task := range readyTasks {
			e.wg.Add(1)
			go func(t *Task) {
				defer e.wg.Done()

				select {
				case semaphore <- struct{}{}: // Acquire semaphore
					defer func() { <-semaphore }() // Release semaphore

					// Execute task
					err := t.Execute(ctx, t.Data)
					result := TaskResult{
						TaskID: t.ID,
						Error:  err,
					}
					e.results <- result

					// Mark task as completed in graph
					e.graph.MarkCompleted(t.ID)

				case <-ctx.Done():
					e.results <- TaskResult{
						TaskID: t.ID,
						Error:  ctx.Err(),
					}
				}
			}(task)
		}

		// Wait for at least one task to complete before checking for more ready tasks
		e.wg.Wait()
	}

	close(e.results)

	// Collect all results
	for result := range e.results {
		allResults = append(allResults, result)
	}

	return allResults
}

// ExecutorBuilder helps build an executor with tasks
type ExecutorBuilder struct {
	executor *Executor
}

// NewExecutorBuilder creates a new builder
func NewExecutorBuilder(maxConcurrency int) *ExecutorBuilder {
	return &ExecutorBuilder{
		executor: NewExecutor(maxConcurrency),
	}
}

// AddTask adds a task to the builder
func (b *ExecutorBuilder) AddTask(id string, data interface{}, execute func(ctx context.Context, data interface{}) error, dependencies ...string) *ExecutorBuilder {
	task := &Task{
		ID:           id,
		Data:         data,
		Execute:      execute,
		Dependencies: dependencies,
	}
	b.executor.AddTask(task)
	return b
}

// Build returns the configured executor
func (b *ExecutorBuilder) Build() *Executor {
	return b.executor
}
