package non_durable_reject

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"

	"github.com/temporalio/features/features/update/updateutil"
	"github.com/temporalio/features/harness/go/harness"
)

const (
	step      = 2
	count     = 5
	updateAdd = "updateActivity"
)

var Feature = harness.Feature{
	Workflows:       NonDurableReject,
	ExpectRunResult: step * count,
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		if reason := updateutil.CheckServerSupportsUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}
		run, err := runner.ExecuteDefault(ctx)
		if err != nil {
			return nil, err
		}

		handle, err := runner.Client.UpdateWorkflow(
			ctx,
			run.GetID(),
			run.GetRunID(),
			updateAdd,
			-1,
		)
		runner.Require.NoError(err)
		var result int
		runner.Require.Error(handle.Get(ctx, &result), "expected negative value to be rejected")

		for i := 0; i < count; i++ {
			handle, err := runner.Client.UpdateWorkflow(
				ctx,
				run.GetID(),
				run.GetRunID(),
				updateAdd,
				step,
			)
			runner.Require.NoError(err)
			runner.Require.NoError(handle.Get(ctx, &result), "expected non-negative value to be accepted")

			handle, err = runner.Client.UpdateWorkflow(
				ctx,
				run.GetID(),
				run.GetRunID(),
				updateAdd,
				-1,
			)
			runner.Require.NoError(err)
			runner.Require.Error(handle.Get(ctx, &result), "expected negative value to be rejected")
		}

		updateutil.RequireNoUpdateRejectedEvents(ctx, runner)
		return run, nil
	},
}

func nonNegative(i int) error {
	if i < 0 {
		return fmt.Errorf("expected non-negative value (%v)", i)
	}
	return nil
}

func NonDurableReject(ctx workflow.Context) (int, error) {
	counter := 0
	if err := workflow.SetUpdateHandlerWithOptions(ctx, updateAdd,
		func(ctx workflow.Context, i int) (int, error) {
			counter += i
			return counter, nil
		},
		workflow.UpdateHandlerOptions{Validator: nonNegative},
	); err != nil {
		return 0, err
	}

	_ = workflow.Sleep(ctx, 4*time.Second)
	return counter, nil
}
