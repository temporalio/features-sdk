package activities

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"
	"unicode"

	"github.com/google/uuid"
	"go.temporal.io/features/features/update/updateutil"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	ChargeAllow ChargeDecision = "CHARGE_ALLOW"
	ChargeDeny  ChargeDecision = "CHARGE_DENY"

	AuthorizationFailedErr  consterr = "authorization failed"
	AuthenticationFailedErr consterr = "authentication failed"
	FraudReportErr          consterr = "failed to send fraud report"
	HoldReleaseErr          consterr = "failed release authorization hold"

	ReadOutputVar = "read_output_var"
)

type (
	ChargeDecision string
	consterr       string

	ChargeOutcome struct {
		Decision ChargeDecision
		Message  string
	}

	Authorization struct{ ID string }
)

var Feature = harness.Feature{
	Workflows: Payment,
	Activities: []any{
		Authorize,
		Authenticate,
		WriteAuditEntry,
		DeepFraudCheck,
		ReportFraud,
		ReleaseHold,
		ScheduleFundRequest,
	},
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		if reason := updateutil.CheckServerSupportsUpdate(ctx, runner.Client); reason != "" {
			return nil, runner.Skip(reason)
		}
		var (
			ccn       = getEnvDefault("PAYMENT_CCN", "0123456789012345")
			subject   = getEnvDefault("PAYMENT_SUBJECT", "John Doe")
			addr      = getEnvDefault("PAYMENT_ADDR", "123 Main St.")
			threshold = must(strconv.ParseFloat(getEnvDefault("PAYMENT_FRAUD_THRESHOLD", "0.67"), 64))
		)

		// Over the next dozen or so lines we (1) start a workflow and then (2)
		// immediately issue an update. These two steps could be combined into
		// UpdateWithStart.
		startopts := client.StartWorkflowOptions{TaskQueue: runner.TaskQueue, WorkflowExecutionTimeout: 2 * time.Minute}
		run, err := runner.Client.ExecuteWorkflow(ctx, startopts, Payment, ccn, subject, addr, threshold)
		if err != nil {
			return nil, err
		}

		// this could be handle := runner.Client.ReadOutputVar(ctx, run.GetID(), run.GetRunID(), "charge_outcome")
		handle, err := runner.Client.UpdateWorkflow(ctx, run.GetID(), run.GetRunID(), ReadOutputVar, "charge_outcome")
		runner.Require.NoError(err)

		var chargeOutcome ChargeOutcome
		runner.Require.NoError(handle.Get(ctx, &chargeOutcome))

		runner.Log.Info("~~> Verification completed", "outcome", chargeOutcome.Decision)
		runner.Log.Info("~~> User-interactive process can be done here, workflow continues in background")
		runner.Log.Info("~~> For the purposes of this test will will stick around to check on the result")

		var processingError temporal.ApplicationError
		runner.Require.NoError(run.Get(ctx, &processingError),
			"Workflow under test is not expected to return an error")
		runner.Require.Equal("", processingError.Error())

		return run, nil
	},
}

func Payment(ctx workflow.Context, ccn, subject, address string, fraudThreshold float64) error {
	setOutput := must(newOutputter(ctx))

	var hold Authorization
	var chargeErr error
	verificationCtx := workflow.WithLocalActivityOptions(ctx, workflow.LocalActivityOptions{
		ScheduleToCloseTimeout: 500 * time.Millisecond,
	})
	sel := workflow.NewNamedSelector(ctx, "charge")
	sel.AddFuture(workflow.ExecuteLocalActivity(verificationCtx, Authorize, ccn),
		func(f workflow.Future) { chargeErr = errors.Join(chargeErr, f.Get(ctx, &hold)) })
	sel.AddFuture(workflow.ExecuteLocalActivity(verificationCtx, Authenticate, subject, address),
		func(f workflow.Future) { chargeErr = errors.Join(chargeErr, f.Get(ctx, nil)) })
	selectN(ctx, sel, 2)
	chargeOutcome := newChargeOutcomeFromError(chargeErr)

	// could be workflow.Output("charge_outcome", chargeOutcome)
	setOutput("charge_outcome", chargeOutcome)

	processingCtx := workflow.WithStartToCloseTimeout(ctx, 60*time.Second)

	if err := workflow.ExecuteActivity(processingCtx, WriteAuditEntry, subject, hold.ID).Get(ctx, nil); err != nil {
		return err
	}

	if len(hold.ID) == 0 {
		return nil
	}

	if chargeOutcome.Decision == ChargeDeny {
		return workflow.ExecuteActivity(processingCtx, ReleaseHold, hold).Get(ctx, nil)
	}
	var fraudScore float64
	if err := workflow.ExecuteActivity(processingCtx, DeepFraudCheck, subject).Get(ctx, &fraudScore); err != nil {
		return err
	}
	if fraudScore <= fraudThreshold {
		return workflow.ExecuteActivity(processingCtx, ScheduleFundRequest, hold).Get(ctx, nil)
	}

	var fraudProcessingErr error
	joinErr := func(f workflow.Future) { fraudProcessingErr = errors.Join(fraudProcessingErr, f.Get(ctx, nil)) }
	sel = workflow.NewNamedSelector(ctx, "fraud_processing")
	sel.AddFuture(workflow.ExecuteActivity(processingCtx, ReportFraud, subject), joinErr).
		AddFuture(workflow.ExecuteActivity(processingCtx, ReleaseHold, hold), joinErr)
	selectN(ctx, sel, 2)

	return fraudProcessingErr
}

func (c consterr) Error() string { return string(c) }

func Authorize(ctx context.Context, ccn string) (Authorization, error) {
	if len(ccn) != 16 {
		err := fmt.Errorf("%w: expected 16 digit CCN, found %v digits", AuthorizationFailedErr, len(ccn))
		activity.GetLogger(ctx).Info("=== authorization failed", "error", err)
		return Authorization{}, temporal.NewNonRetryableApplicationError(err.Error(), "EBADCCN", err)
	}
	if ccn[0] != '0' {
		err := fmt.Errorf("%w: expected first digit to be '0', found '%c'", AuthorizationFailedErr, ccn[0])
		activity.GetLogger(ctx).Info("=== authorization failed", "error", err)
		return Authorization{}, temporal.NewNonRetryableApplicationError(err.Error(), "EBADCCN", err)
	}
	id := uuid.NewString()
	activity.GetLogger(ctx).Info("=== authorization successful", "hold-id", id)
	return Authorization{ID: id}, nil
}

func Authenticate(ctx context.Context, subject string, address string) error {
	if len(address) == 0 || !unicode.IsDigit([]rune(address)[0]) {
		err := fmt.Errorf("%w: address %q for subject %q seems bogus", AuthenticationFailedErr, address, subject)
		activity.GetLogger(ctx).Info("=== authentication failed", "error", err)
		return temporal.NewNonRetryableApplicationError(err.Error(), "EBADADDR", err)
	}
	activity.GetLogger(ctx).Info("=== authentication successful")
	return nil
}

func WriteAuditEntry(ctx context.Context, subject, authzID string) error {
	time.Sleep(5 * time.Second)
	activity.GetLogger(ctx).Info("=== wrote audit log", "subject", subject, "authorization-hold-id", authzID)
	return nil
}

func DeepFraudCheck(ctx context.Context, subject string) (float64, error) {
	time.Sleep(5 * time.Second)
	score := rand.Float64()
	activity.GetLogger(ctx).Info("=== fraud score calculated", "subject", subject, "score", score)
	return score, nil
}

func ReportFraud(ctx context.Context, subject string) error {
	const threshold = 0.6
	time.Sleep(5 * time.Second)
	if chance := rand.Float64(); chance > threshold {
		return fmt.Errorf("=== %w: bad luck (%v > %v)", FraudReportErr, chance, threshold)
	}
	activity.GetLogger(ctx).Info("=== fraud report sent", "subject", subject)
	return nil
}

func ReleaseHold(ctx context.Context, hold Authorization) error {
	const threshold = 0.75
	time.Sleep(3 * time.Second)
	if chance := rand.Float64(); chance > 0.75 {
		return fmt.Errorf("=== %w: bad luck (%v > %v)", HoldReleaseErr, chance, threshold)
	}
	activity.GetLogger(ctx).Info("=== authorization hold released", "authorization-hold-id", hold.ID)
	return nil
}

func ScheduleFundRequest(ctx context.Context, hold Authorization) error {
	time.Sleep(3 * time.Second)
	activity.GetLogger(ctx).Info("=== scheduled fund request into batch", "authorization-hold-id", hold.ID)
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

func getEnvDefault(name, defval string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return defval
}

func newChargeOutcomeFromError(err error) ChargeOutcome {
	if err == nil {
		return ChargeOutcome{Decision: ChargeAllow}
	}
	return ChargeOutcome{Decision: ChargeDeny, Message: err.Error()}
}

func selectN(ctx workflow.Context, sel workflow.Selector, n int) {
	for i := 0; i < n; i++ {
		sel.Select(ctx)
	}
}
