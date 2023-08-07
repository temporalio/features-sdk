package activities

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"go.temporal.io/features/features/update/updateutil"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const (
	syncPart            = "synchronous_part"
	expectedSyncResult  = 1337
	expectedErrorString = "Oops"
)

var Feature = harness.Feature{
	Workflows:  MidWorkflowReturn,
	Activities: FiftyPercentErrorActivity,
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		if reason := updateutil.CheckServerSupportsUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}

		// Over the next dozen or so lines we (1) start a workflow and then (2)
		// immediately issue an update. These two steps could be combined into
		// UpdateWithStart.
		run, err := runner.ExecuteDefault(ctx)
		if err != nil {
			return nil, err
		}
		handle, err := runner.Client.UpdateWorkflow(
			ctx,
			run.GetID(),
			run.GetRunID(),
			syncPart,
		)
		runner.Require.NoError(err)

		var (
			updateResult    int
			cleanupOccurred bool
		)
		updateErr := handle.Get(ctx, &updateResult)

		runner.Require.NoError(run.Get(ctx, &cleanupOccurred),
			"Workflow under test is not expected to return an error")
		runner.Require.Equal(cleanupOccurred, updateErr != nil,
			"If the update returned an error, then cleanup should have occurred")

		return run, nil
	},
}

// FiftyPercentErrorActivity returns an error for ~half of all invocations
func FiftyPercentErrorActivity(ctx context.Context) (int, error) {
	time.Sleep(100 * time.Millisecond)
	if rand.Float64() > 0.5 {
		return 0, errors.New(expectedErrorString)
	}
	return expectedSyncResult, nil
}

func MidWorkflowReturn(ctx workflow.Context) (bool, error) {
	log := workflow.GetLogger(ctx)

	// Update writes a bool to this channel to indicate whether or not it wants
	// cleanup to occur.
	cleanupChan := workflow.NewChannel(ctx)

	if err := workflow.SetUpdateHandler(ctx, syncPart,
		func(ctx workflow.Context) (_ int, retErr error) {
			// sent true to cleanupChan if retErr is not nil to trigger
			// background cleanup. Send false otherwise.
			defer func() { cleanupChan.Send(ctx, retErr != nil) }()

			log.Info("Imma do some synchronous stuff that might fail")
			aopts := workflow.ActivityOptions{
				StartToCloseTimeout: 5 * time.Second,
				RetryPolicy:         harness.RetryDisabled,
			}
			act := workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, aopts), FiftyPercentErrorActivity)
			result := 0
			if err := act.Get(ctx, &result); err != nil {
				log.Info("activity failed, requesting cleanup")
				return 0, err
			}
			return result, nil
		},
	); err != nil {
		return false, err
	}

	var runCleanup bool
	_ = cleanupChan.Receive(ctx, &runCleanup)

	if !runCleanup {
		log.Info("background cleanup IS NOT needed")
		return false, nil
	}

	log.Info("background cleanup IS needed")
	log.Info("sleep(3s) to pretend to clean up")
	if err := workflow.Sleep(ctx, 3*time.Second); err != nil {
		return false, err
	}
	return true, nil
}
