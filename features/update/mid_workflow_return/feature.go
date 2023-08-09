package activities

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"go.temporal.io/features/features/update/updateutil"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const ReadOutputVar = "read_output_var"

var Feature = harness.Feature{
	Workflows:  Payment,
	Activities: []any{VerifyCCN, CheckRisk, RequestFundsTransfer, ReportToGovernment, AuditTransaction},
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		if reason := updateutil.CheckServerSupportsUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}
		startopts := client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: 2 * time.Minute,
		}

		// Over the next dozen or so lines we (1) start a workflow and then (2)
		// immediately issue an update. These two steps could be combined into
		// UpdateWithStart.
		run, err := runner.Client.ExecuteWorkflow(ctx, startopts, Payment, "mpm", "0111111111111111", 0.67)
		if err != nil {
			return nil, err
		}
		handle, err := runner.Client.UpdateWorkflow(ctx, run.GetID(), run.GetRunID(), ReadOutputVar, "verification_outcome")
		runner.Require.NoError(err)

		var summary VerificationSummary
		runner.Require.NoError(handle.Get(ctx, &summary))

		if !summary.AllPassed() {
			runner.Log.Info("~~> Verification failed so returning early", "summary", summary)
			return run, nil
		}

		runner.Log.Info("~~> Verification completed successfully")
		runner.Log.Info("~~> WF will now execute the transaction in the background")
		runner.Log.Info("~~> For the purposes of this test will will stick around to check on the result")

		var processingError temporal.ApplicationError
		runner.Require.NoError(run.Get(ctx, &processingError),
			"Workflow under test is not expected to return an error")
		runner.Require.Equal("", processingError.Error())

		return run, nil
	},
}

type VerificationDecision string

const (
	VerificationPassed VerificationDecision = "PASSED"
	VerificationFailed VerificationDecision = "FAILED"
)

type VerificationOutcome struct {
	Decision VerificationDecision
	Message  string
	Descr    string
}

type VerificationSummary struct {
	Outcomes []VerificationOutcome
}

func (vs *VerificationSummary) String() string {
	var buf strings.Builder
	buf.WriteString("[")
	for _, check := range vs.Outcomes {
		buf.WriteString("descr='")
		buf.WriteString(check.Descr)
		buf.WriteString("'; result='")
		buf.WriteString(string(check.Decision))
		buf.WriteString("'; message='")
		buf.WriteString(check.Message)
	}
	buf.WriteString("']")
	return buf.String()
}

func (vs *VerificationSummary) AllPassed() bool {
	for _, check := range vs.Outcomes {
		if check.Decision != VerificationPassed {
			return false
		}
	}
	return true
}

func VerifyCCN(ctx context.Context, ccn string) (VerificationOutcome, error) {
	outcome := VerificationOutcome{
		Decision: VerificationPassed,
		Descr:    "CCN verfication",
	}
	if len(ccn) != 16 {
		outcome.Decision = VerificationFailed
		outcome.Message = fmt.Sprintf("expected 16 digits, found %v", len(ccn))
	}
	if ccn[0] != '0' {
		outcome.Decision = VerificationFailed
		outcome.Message = fmt.Sprintf("expected first digit to be '0', found '%c'", ccn[0])
	}
	return outcome, nil
}

func CheckRisk(ctx context.Context, subject string, threshold float64) (VerificationOutcome, error) {
	outcome := VerificationOutcome{
		Decision: VerificationPassed,
		Descr:    "Risk check",
	}
	if risk := rand.Float64(); risk > threshold {
		outcome.Decision = VerificationFailed
		outcome.Message = fmt.Sprintf("risk rating of %v for subject %v exceeds threshold of %v", risk, subject, threshold)
	}
	return outcome, nil
}

func RequestFundsTransfer(ctx context.Context, ccn string) error {
	time.Sleep(6 * time.Second)
	if rand.Float64() > 0.5 {
		return errors.New("=== fund transfer failed because sometimes it's like that")
	}
	activity.GetLogger(ctx).Info("=== funds transferred")
	return nil
}

func ReportToGovernment(ctx context.Context, subject string) error {
	time.Sleep(2 * time.Second)
	activity.GetLogger(ctx).Info("=== activity reported to government", "subject", subject)
	return nil
}

func AuditTransaction(ctx context.Context) error {
	time.Sleep(4 * time.Second)
	activity.GetLogger(ctx).Info("=== transaction written to audit log")
	return nil
}

func Payment(ctx workflow.Context, subject, ccn string, riskThreshold float64) error {
	workflowOutput := must(newOutputter(ctx))

	var summary VerificationSummary
	var verificationErr error
	verificationCtx := workflow.WithLocalActivityOptions(ctx, workflow.LocalActivityOptions{
		StartToCloseTimeout: 500 * time.Millisecond,
	})
	verificationCallback := func(f workflow.Future) {
		var outcome VerificationOutcome
		if err := f.Get(ctx, &outcome); err != nil {
			verificationErr = errors.Join(verificationErr, err)
			return
		}
		summary.Outcomes = append(summary.Outcomes, outcome)
	}
	sel := workflow.NewNamedSelector(ctx, "verification")
	sel.AddFuture(workflow.ExecuteLocalActivity(verificationCtx, VerifyCCN, ccn), verificationCallback).
		AddFuture(workflow.ExecuteLocalActivity(verificationCtx, CheckRisk, subject, riskThreshold), verificationCallback)
	sel.Select(ctx)
	sel.Select(ctx)

	if verificationErr != nil {
		return verificationErr
	}

	workflowOutput("verification_outcome", summary)

	if !summary.AllPassed() {
		return nil
	}

	var processingErr error
	joinErr := func(f workflow.Future) { processingErr = errors.Join(processingErr, f.Get(ctx, nil)) }
	processingCtx := workflow.WithStartToCloseTimeout(ctx, 60*time.Second)
	sel = workflow.NewNamedSelector(ctx, "processing")
	sel.AddFuture(workflow.ExecuteActivity(processingCtx, RequestFundsTransfer, ccn), joinErr).
		AddFuture(workflow.ExecuteActivity(processingCtx, ReportToGovernment, subject), joinErr).
		AddFuture(workflow.ExecuteActivity(processingCtx, AuditTransaction), joinErr)
	for i := 0; i < 3; i++ {
		sel.Select(ctx)
		if processingErr != nil {
			return processingErr
		}
	}
	return nil
}

// Outputter is a func that can be used to export workflow "attributes"
// which are named dataflow variables, expressed as Futures.
type Outputter func(string, any)

// newOutputter creates an Outputer and registers an Update handler to expose
// the workflow outputs via the same. The Update handler takes as a parameter
// the name of the attribute to be observed and uses a validation handler to
// check that the requested attribute is part of the exported set before
// processing the read.
func newOutputter(ctx workflow.Context) (Outputter, error) {
	attrs := map[string]any{}

	handler := func(ctx workflow.Context, attrName string) (any, error) {
		workflow.Await(ctx, func() bool {
			_, ok := attrs[attrName]
			return ok
		})
		v := attrs[attrName]
		return v, nil
	}
	opts := workflow.UpdateHandlerOptions{}

	if err := workflow.SetUpdateHandlerWithOptions(ctx, ReadOutputVar, handler, opts); err != nil {
		return nil, err
	}

	return func(name string, val any) {
		if _, found := attrs[name]; !found {
			attrs[name] = val
		}
	}, nil
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}
