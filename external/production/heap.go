package production

import (
	"container/heap"
	"time"
)

// jobHeap implements a min-heap of jobs ordered by EndTime.
// Jobs completing soonest are at the top of the heap.
type jobHeap []*Job

func (h jobHeap) Len() int {
	return len(h)
}

func (h jobHeap) Less(i, j int) bool {
	// Earlier end time = higher priority (min-heap)
	return h[i].EndTime.Before(h[j].EndTime)
}

func (h jobHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *jobHeap) Push(x any) {
	*h = append(*h, x.(*Job))
}

func (h *jobHeap) Pop() any {
	old := *h
	n := len(old)
	job := old[n-1]
	old[n-1] = nil // Avoid memory leak
	*h = old[0 : n-1]
	return job
}

// Peek returns the job with the earliest end time without removing it.
// Returns nil if heap is empty.
func (h *jobHeap) Peek() *Job {
	if len(*h) == 0 {
		return nil
	}
	return (*h)[0]
}

// Remove removes a job from the heap by ID. Returns true if found and removed.
func (h *jobHeap) Remove(id JobID) bool {
	for i, job := range *h {
		if job.ID == id {
			heap.Remove(h, i)
			return true
		}
	}
	return false
}

// newJobHeap creates an empty job heap.
func newJobHeap() *jobHeap {
	h := &jobHeap{}
	heap.Init(h)
	return h
}

// processCompletedJobs extracts all jobs that have completed by the given time.
// Returns them in completion order (earliest first).
func (h *jobHeap) processCompletedJobs(now time.Time) []*Job {
	var completed []*Job

	for {
		job := h.Peek()
		if job == nil {
			break
		}

		// If the earliest job isn't complete yet, none are
		if now.Before(job.EndTime) {
			break
		}

		// Remove and collect completed job
		heap.Pop(h)
		completed = append(completed, job)
	}

	return completed
}
