package activities

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"go.temporal.io/features/features/update/updateutil"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const (
	watchAttr           = "watch_attribute"
	expectedSyncResult  = 1337
	expectedErrorString = "Oops"
	syncResultAttr      = "sync_result"
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
			watchAttr,
			syncResultAttr,
		)
		runner.Require.NoError(err)

		var (
			updateResult    int
			cleanupOccurred bool
		)
		updateErr := handle.Get(ctx, &updateResult)

		runner.Require.NoError(run.Get(ctx, &cleanupOccurred),
			"Workflow under test is not expected to return an error")

		if updateErr == nil {
			runner.Require.Equal(expectedSyncResult, updateResult)
		} else {
			runner.Require.ErrorContains(updateErr, expectedErrorString)
		}

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

	syncResult, syncResultSetter := workflow.NewFuture(ctx)
	observableAttributes := map[string]workflow.Future{syncResultAttr: syncResult}

	if err := workflow.SetUpdateHandlerWithOptions(ctx, watchAttr,
		func(ctx workflow.Context, attrName string) (interface{}, error) {
			var out interface{}
			if err := observableAttributes[attrName].Get(ctx, &out); err != nil {
				return nil, err
			}
			return out, nil
		},
		workflow.UpdateHandlerOptions{
			Validator: func(attrName string) error {
				_, ok := observableAttributes[attrName]
				if !ok {
					return fmt.Errorf("attribute %q not found", attrName)
				}
				return nil
			},
		},
	); err != nil {
		return false, err
	}

	log.Info("run an activity that fails 50% of the time")
	aopts := workflow.LocalActivityOptions{
		StartToCloseTimeout: 5 * time.Second,
		RetryPolicy:         harness.RetryDisabled,
	}
	act := workflow.ExecuteLocalActivity(workflow.WithLocalActivityOptions(ctx, aopts), FiftyPercentErrorActivity)
	syncResultSetter.Chain(act)

	err := act.Get(ctx, nil)
	if err == nil {
		log.Info("activity succeeded - background cleanup IS NOT needed")
		return false, nil
	}
	log.Info("activity failed - background cleanup IS needed")
	log.Info("sleep(2s) to pretend to clean up")
	if err := workflow.Sleep(ctx, 2*time.Second); err != nil {
		return false, err
	}
	return true, nil
}
