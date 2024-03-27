package server_restarted_for_initiator

import (
	"context"
	"time"

	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows: Workflow,
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		if err := runner.ProxyRestart(ctx, 2*time.Second, true); err != nil {
			return nil, err
		}

		opts := client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: 1 * time.Minute,
			RetryPolicy: &temporal.RetryPolicy{
				InitialInterval:    1 * time.Millisecond,
				MaximumInterval:    100 * time.Millisecond,
				BackoffCoefficient: 2.0,
			},
		}
		return runner.Client.ExecuteWorkflow(ctx, opts, Workflow)
	},
}

func Workflow(ctx workflow.Context) (string, error) {
	return "OK", nil
}
