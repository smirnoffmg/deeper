# Worker Pool Architecture

This document outlines the design and implementation of the worker pool system in Deeper, which is responsible for managing concurrent operations and ensuring that the application operates within its configured concurrency limits.

## 1. Overview

The worker pool is a bounded concurrency system designed to replace the previous model of unlimited goroutine creation. It provides a centralized mechanism for managing concurrent tasks, ensuring that the total number of active goroutines is controlled and predictable. This prevents resource exhaustion and improves the overall stability and performance of the application.

## 2. Design

The worker pool follows the classic worker pool pattern, consisting of the following components:

- **Job Queue:** A buffered channel that holds jobs to be processed.
- **Workers:** A fixed number of goroutines that pull jobs from the queue and execute them.
- **Job:** A struct that represents a task to be executed, including the function to execute and a callback to handle the result.

### 2.1. Key Features

- **Configurable Concurrency:** The number of workers in the pool is determined by the `DEEPER_MAX_CONCURRENCY` environment variable, allowing users to configure the concurrency limit based on their system's resources.
- **Centralized Management:** All concurrent operations, including trace processing and plugin execution, are managed by the central worker pool.
- **Graceful Shutdown:** The worker pool supports graceful shutdown, ensuring that all pending jobs are completed before the application exits.

## 3. Integration

The worker pool is integrated into the application using the `uber-fx` dependency injection framework.

- **Provider:** A `provideWorkerPool` function in `internal/app/deeper/app.go` creates a new instance of the worker pool and provides it to the application's dependency graph.
- **Lifecycle Hooks:** The worker pool is started and stopped using `fx.Lifecycle` hooks, ensuring that it's managed correctly within the application's lifecycle.
- **Injection:** The worker pool is injected into the `Engine` and `Processor` components, allowing them to submit jobs for concurrent execution.

## 4. Workflow

1.  **Initialization:** The worker pool is initialized at application startup with a number of workers equal to `config.MaxConcurrency`.
2.  **Job Submission:** The `Engine` and `Processor` submit jobs to the worker pool's job queue.
3.  **Job Execution:** The workers pull jobs from the queue and execute them concurrently.
4.  **Result Handling:** The results of each job are passed to a callback function, which handles the results and any errors that may have occurred.
5.  **Shutdown:** When the application shuts down, the worker pool's `Stop` method is called, which closes the job queue and waits for all workers to finish their current jobs.

## 5. Benefits

- **Bounded Concurrency:** The worker pool ensures that the number of concurrent operations never exceeds the configured limit.
- **Improved Performance:** By limiting concurrency, the worker pool prevents the application from being overloaded with too many goroutines, which can lead to performance degradation.
- **Increased Stability:** The worker pool improves the stability of the application by preventing resource exhaustion and ensuring that the application can handle a large number of traces without crashing.
- **Simplified Concurrency Management:** The worker pool provides a centralized and simplified approach to concurrency management, making the code easier to understand and maintain.
