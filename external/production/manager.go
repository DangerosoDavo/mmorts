package production

import (
	"container/heap"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitas-015/inventory"
)

// Manager handles production jobs for a partition.
// It is the core tool that external systems orchestrate.
type Manager struct {
	id              string
	registry        *RecipeRegistry
	inventories     InventoryProvider
	eventBus        EventBus
	modifierSources []ModifierSource

	mu         sync.RWMutex
	jobs       map[JobID]*Job
	activeJobs *jobHeap
	lastUpdate time.Time
	nextJobID  int64
}

// NewManager creates a new production manager.
func NewManager(
	id string,
	registry *RecipeRegistry,
	inventories InventoryProvider,
	eventBus EventBus,
	modifierSources []ModifierSource,
) *Manager {
	return &Manager{
		id:              id,
		registry:        registry,
		inventories:     inventories,
		eventBus:        eventBus,
		modifierSources: modifierSources,
		jobs:            make(map[JobID]*Job),
		activeJobs:      newJobHeap(),
		lastUpdate:      time.Now(),
	}
}

// ID returns the manager's identifier.
func (m *Manager) ID() string {
	return m.id
}

// StartProduction initiates a new production job.
// Inputs are IMMEDIATELY consumed from inventory atomically.
// Returns JobID on success, error if insufficient resources or invalid recipe.
func (m *Manager) StartProduction(recipeID RecipeID, ownerID inventory.OwnerID, inventoryID string) (JobID, error) {
	return m.startProductionInternal(recipeID, ownerID, inventoryID, false)
}

// StartRepeatingProduction initiates a repeating production job that runs until stopped.
// Each cycle consumes inputs and produces outputs, then automatically restarts.
// Returns JobID on success, error if insufficient resources or invalid recipe.
func (m *Manager) StartRepeatingProduction(recipeID RecipeID, ownerID inventory.OwnerID, inventoryID string) (JobID, error) {
	return m.startProductionInternal(recipeID, ownerID, inventoryID, true)
}

// startProductionInternal is the internal implementation for starting production.
func (m *Manager) startProductionInternal(recipeID RecipeID, ownerID inventory.OwnerID, inventoryID string, repeat bool) (JobID, error) {
	// 1. Lookup recipe
	recipe := m.registry.Lookup(recipeID)
	if recipe == nil {
		return "", fmt.Errorf("recipe not found: %s", recipeID)
	}

	// 2. Resolve modifiers
	modifiers := m.resolveModifiers(ownerID, recipeID)

	// 3. Apply modifiers to calculate effective values
	effectiveInputs := applyInputModifiers(recipe.Inputs, modifiers.InputCost)
	effectiveOutputs := applyOutputModifiers(recipe.Outputs, modifiers.OutputYield)
	effectiveDuration := time.Duration(applyDurationModifier(int64(recipe.Duration), modifiers.TimeSpeed))

	// 4. Get inventory
	inv, err := m.inventories.GetInventory(inventoryID)
	if err != nil {
		return "", fmt.Errorf("inventory not found: %w", err)
	}

	// 5. IMMEDIATELY consume inputs (atomic operation)
	if err := m.inventories.ConsumeItems(inv, effectiveInputs); err != nil {
		return "", fmt.Errorf("insufficient resources: %w", err)
	}

	// 6. Create job
	now := time.Now()
	jobID := m.generateJobID()

	job := &Job{
		ID:                jobID,
		Recipe:            recipeID,
		Owner:             ownerID,
		InventoryID:       inventoryID,
		State:             JobRunning,
		Progress:          0.0,
		StartTime:         now,
		EndTime:           now.Add(effectiveDuration),
		InputSnapshot:     effectiveInputs,
		Modifiers:         modifiers,
		EffectiveInputs:   effectiveInputs,
		EffectiveOutputs:  effectiveOutputs,
		EffectiveDuration: effectiveDuration,
		Repeat:            repeat,
		CyclesCompleted:   0,
		Context:           make(map[string]any),
	}

	// 7. Add to active jobs
	m.mu.Lock()
	m.jobs[jobID] = job
	heap.Push(m.activeJobs, job)
	m.mu.Unlock()

	// 8. Emit event
	m.eventBus.Publish(Event{
		Type:      EventJobStarted,
		Job:       job,
		Timestamp: now,
	})

	return jobID, nil
}

// Update processes completed jobs up to the given time.
// Call this from your game loop or ECS system.
func (m *Manager) Update(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastUpdate = now

	// Process all completed jobs
	completed := m.activeJobs.processCompletedJobs(now)

	for _, job := range completed {
		m.completeJob(job, now)
	}
}

// completeJob handles job completion (caller must hold lock).
func (m *Manager) completeJob(job *Job, now time.Time) {
	// Get inventory
	inv, err := m.inventories.GetInventory(job.InventoryID)
	if err != nil {
		// Failed to get inventory - mark job as failed
		job.State = JobFailed
		job.Progress = 1.0

		m.eventBus.Publish(Event{
			Type:      EventJobFailed,
			Job:       job,
			Timestamp: now,
			Data: map[string]any{
				"error": err.Error(),
			},
		})

		delete(m.jobs, job.ID)
		return
	}

	// Roll probabilistic outputs
	actualOutputs := m.rollOutputs(job.EffectiveOutputs)

	// Add outputs to inventory
	if err := m.inventories.AddItems(inv, actualOutputs); err != nil {
		// Failed to add items (inventory full, etc.) - mark as failed
		job.State = JobFailed
		job.Progress = 1.0

		m.eventBus.Publish(Event{
			Type:      EventJobFailed,
			Job:       job,
			Timestamp: now,
			Data: map[string]any{
				"error": err.Error(),
			},
		})

		delete(m.jobs, job.ID)
		return
	}

	// Success - increment cycle counter
	job.CyclesCompleted++
	job.State = JobComplete
	job.Progress = 1.0

	m.eventBus.Publish(Event{
		Type:      EventJobCompleted,
		Job:       job,
		Timestamp: now,
		Data: map[string]any{
			"cyclesCompleted": job.CyclesCompleted,
		},
	})

	// Check if job should repeat
	if job.Repeat {
		// Try to restart the job
		if err := m.restartRepeatingJob(job, now); err != nil {
			// Failed to restart (insufficient resources, etc.)
			// Job stops repeating
			m.eventBus.Publish(Event{
				Type:      EventJobFailed,
				Job:       job,
				Timestamp: now,
				Data: map[string]any{
					"error":           err.Error(),
					"reason":          "failed_to_restart",
					"cyclesCompleted": job.CyclesCompleted,
				},
			})
			delete(m.jobs, job.ID)
		}
		// Job was restarted successfully, stays in m.jobs
	} else {
		// One-time job - remove from active jobs
		delete(m.jobs, job.ID)
	}
}

// restartRepeatingJob attempts to restart a repeating job (caller must hold lock).
func (m *Manager) restartRepeatingJob(job *Job, now time.Time) error {
	// Get inventory
	inv, err := m.inventories.GetInventory(job.InventoryID)
	if err != nil {
		return fmt.Errorf("inventory not found: %w", err)
	}

	// Try to consume inputs for next cycle
	if err := m.inventories.ConsumeItems(inv, job.EffectiveInputs); err != nil {
		return fmt.Errorf("insufficient resources for next cycle: %w", err)
	}

	// Reset job for next cycle
	job.State = JobRunning
	job.Progress = 0.0
	job.StartTime = now
	job.EndTime = now.Add(job.EffectiveDuration)

	// Re-add to heap
	heap.Push(m.activeJobs, job)

	// Emit restart event (reuse JobStarted event type)
	m.eventBus.Publish(Event{
		Type:      EventJobStarted,
		Job:       job,
		Timestamp: now,
		Data: map[string]any{
			"isRestart":       true,
			"cyclesCompleted": job.CyclesCompleted,
		},
	})

	return nil
}

// rollOutputs applies probability to outputs and returns actual items produced.
func (m *Manager) rollOutputs(outputs []ItemYield) []ItemYield {
	if len(outputs) == 0 {
		return nil
	}

	result := make([]ItemYield, 0, len(outputs))

	for _, output := range outputs {
		// Roll for probability
		if output.Probability >= 1.0 || rand.Float64() < output.Probability {
			result = append(result, output)
		}
	}

	return result
}

// CancelProduction cancels an active job.
// By default, does NOT refund items (application can implement refund logic separately).
func (m *Manager) CancelProduction(jobID JobID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return errors.New("job not found")
	}

	if job.State != JobRunning {
		return fmt.Errorf("job is not running: %s", job.State)
	}

	// Remove from active jobs heap
	m.activeJobs.Remove(jobID)

	// Update job state
	job.State = JobCancelled
	job.Progress = job.CalculateProgress(time.Now())

	// Emit event
	m.eventBus.Publish(Event{
		Type:      EventJobCancelled,
		Job:       job,
		Timestamp: time.Now(),
	})

	delete(m.jobs, jobID)

	return nil
}

// CancelProductionWithRefund cancels a job and refunds all input items.
func (m *Manager) CancelProductionWithRefund(jobID JobID) error {
	m.mu.Lock()

	job, exists := m.jobs[jobID]
	if !exists {
		m.mu.Unlock()
		return errors.New("job not found")
	}

	if job.State != JobRunning {
		m.mu.Unlock()
		return fmt.Errorf("job is not running: %s", job.State)
	}

	// Get inventory before unlocking
	inventoryID := job.InventoryID
	inputSnapshot := job.InputSnapshot

	m.mu.Unlock()

	// Refund items (outside lock to avoid potential deadlock with inventory operations)
	inv, err := m.inventories.GetInventory(inventoryID)
	if err != nil {
		return fmt.Errorf("failed to get inventory for refund: %w", err)
	}

	// Convert ItemRequirements to ItemYields for refund
	refundItems := make([]ItemYield, 0, len(inputSnapshot))
	for _, req := range inputSnapshot {
		if req.Consume {
			refundItems = append(refundItems, ItemYield{
				Item:        req.Item,
				Quantity:    req.Quantity,
				Probability: 1.0,
			})
		}
	}

	if err := m.inventories.AddItems(inv, refundItems); err != nil {
		return fmt.Errorf("failed to refund items: %w", err)
	}

	// Now cancel the job
	return m.CancelProduction(jobID)
}

// GetJob retrieves a job by ID. Returns nil if not found.
func (m *Manager) GetJob(jobID JobID) *Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job := m.jobs[jobID]
	if job != nil {
		// Update progress before returning
		job.Progress = job.CalculateProgress(time.Now())
	}
	return job
}

// GetActiveJobs returns all jobs for a specific owner.
func (m *Manager) GetActiveJobs(ownerID inventory.OwnerID) []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Job, 0)
	now := time.Now()

	for _, job := range m.jobs {
		if job.Owner == ownerID && job.State == JobRunning {
			// Update progress
			job.Progress = job.CalculateProgress(now)
			result = append(result, job)
		}
	}

	return result
}

// GetAllJobs returns all active jobs in this manager.
func (m *Manager) GetAllJobs() []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Job, 0, len(m.jobs))
	now := time.Now()

	for _, job := range m.jobs {
		// Update progress
		job.Progress = job.CalculateProgress(now)
		result = append(result, job)
	}

	return result
}

// JobCount returns the number of active jobs.
func (m *Manager) JobCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.jobs)
}

// resolveModifiers combines all modifier sources for a job.
func (m *Manager) resolveModifiers(ownerID inventory.OwnerID, recipeID RecipeID) Modifiers {
	result := DefaultModifiers()

	for _, source := range m.modifierSources {
		mods := source.GetModifiers(ownerID, recipeID)
		result = result.Combine(mods)
	}

	return result
}

// generateJobID generates a unique job ID for this manager.
func (m *Manager) generateJobID() JobID {
	id := atomic.AddInt64(&m.nextJobID, 1)
	return JobID(fmt.Sprintf("%s-%d", m.id, id))
}
