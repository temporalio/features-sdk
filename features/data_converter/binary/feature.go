package binary

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/temporalio/features/harness/go/harness"
	common "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

var expectedResult = []byte{0xde, 0xad, 0xbe, 0xef}

var Feature = harness.Feature{
	Workflows:   Workflow,
	CheckResult: CheckResult,
	// ExecuteDefault does not support workflow arguments
	Execute: harness.ExecuteWithArgs(Workflow, expectedResult),
}

// run an echo workflow that returns binary value `0xdeadbeef`
func Workflow(ctx workflow.Context, res []byte) ([]byte, error) {
	return res, nil
}

func CheckResult(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
	// verify client result is binary `0xdeadbeef`
	result := make([]byte, 4)
	if err := run.Get(ctx, &result); err != nil {
		return err
	}
	if !bytes.Equal(result, expectedResult) {
		return fmt.Errorf("invalid result: %v", result)
	}

	payload, err := harness.GetWorkflowResultPayload(ctx, runner.Client, run.GetID())
	if err != nil {
		return err
	}

	// load JSON payload from `./payload.json` and compare it to result payload
	file, err := os.Open(filepath.Join(runner.Feature.AbsDir, "../../../features/data_converter/binary/payload.json"))
	if err != nil {
		return err
	}

	expectedPayload := &common.Payload{}
	decoder := json.NewDecoder(file)
	var obj json.RawMessage
	if err := decoder.Decode(&obj); err != nil {
		return err
	}
	err = protojson.Unmarshal(obj, expectedPayload)
	if err != nil {
		return err
	}
	runner.Require.Equal(expectedPayload, payload)

	payloadArg, err := harness.GetWorkflowArgumentPayload(ctx, runner.Client, run.GetID())
	if err != nil {
		return err
	}

	runner.Require.Equal(payload, payloadArg)

	return nil
}
