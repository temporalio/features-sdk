package update.async_accepted;

import io.temporal.activity.ActivityInterface;
import io.temporal.client.UpdateHandle;
import io.temporal.client.UpdateOptions;
import io.temporal.client.WorkflowUpdateTimeoutOrCancelledException;
import io.temporal.client.WorkflowUpdateException;
import io.temporal.client.WorkflowUpdateStage;
import io.temporal.client.WorkflowUpdateTimeoutOrCancelledException;
import io.temporal.failure.ApplicationFailure;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.sdkfeatures.SimpleWorkflow;
import io.temporal.workflow.SignalMethod;
import io.temporal.workflow.UpdateMethod;
import io.temporal.workflow.Workflow;
import java.time.Duration;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Assertions;

@ActivityInterface
public interface feature extends Feature, SimpleWorkflow {
  Duration SLEEP_TIMEOUT = Duration.ofSeconds(2);
  int UPDATE_RESULT = 123;

  @UpdateMethod
  Integer update(Boolean sleep);

  @SignalMethod
  void finish();

  class Impl implements feature {

    private boolean doFinish = false;

    @Override
    public void workflow() {
      Workflow.await(() -> this.doFinish);
    }

    @Override
    public Integer update(Boolean sleep) {
      if (sleep) {
        Workflow.sleep(SLEEP_TIMEOUT);
      } else {
        throw ApplicationFailure.newFailure("I was told I should fail", "Failure");
      }
      return UPDATE_RESULT;
    }

    @Override
    public void finish() {
      this.doFinish = true;
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      runner.skipIfAsyncAcceptedUpdateNotSupported();

      var run = runner.executeSingleParameterlessWorkflow();
      var stub = runner.client.newWorkflowStub(feature.class, run.execution.getWorkflowId());

      var untypedStub =
          runner.client.newUntypedWorkflowStub(
              run.execution.getWorkflowId(),
              Optional.of(run.execution.getRunId()),
              Optional.empty());

      // Issue an async update that should succeed after SLEEP_TIMEOUT
      var updateId = UUID.randomUUID().toString();
      UpdateHandle<Integer> handle =
          untypedStub.startUpdate(
              UpdateOptions.newBuilder(Integer.class)
                  .setUpdateName("update")
                  .setUpdateId(updateId)
                  .setFirstExecutionRunId(run.execution.getRunId())
                  .setWaitForStage(WorkflowUpdateStage.ACCEPTED)
                  .build(),
              true);

      // Create a separate handle to the same update
      UpdateHandle<Integer> otherHandle = untypedStub.getUpdateHandle(updateId, Integer.class);
      // should block on in-flight update
      Assertions.assertEquals(UPDATE_RESULT, otherHandle.getResultAsync().get());
      Assertions.assertEquals(UPDATE_RESULT, handle.getResultAsync().get());
      // issue an async update that should throw
      updateId = UUID.randomUUID().toString();
      try {
        // If the worker accepts the update, but fails it in the same workflow task
        // the update will be marked as failed and the exception may be thrown
        // from startUpdate. This is not consistent with the behavior of the
        // other SDKs.
        UpdateHandle<Integer> errorHandle =
            untypedStub.startUpdate(
                UpdateOptions.newBuilder(Integer.class)
                    .setUpdateName("update")
                    .setUpdateId(updateId)
                    .setFirstExecutionRunId(run.execution.getRunId())
                    .setWaitForStage(WorkflowUpdateStage.ACCEPTED)
                    .build(),
                false);
        errorHandle.getResultAsync().get();
        Assertions.fail("unreachable");
      } catch (Throwable e) {
        if (e instanceof ExecutionException) {
          e = e.getCause();
        }
        Assertions.assertTrue(e instanceof WorkflowUpdateException);
        WorkflowUpdateException wue = (WorkflowUpdateException) e;
        Assertions.assertTrue(wue.getCause() instanceof ApplicationFailure);
        Assertions.assertEquals("Failure", ((ApplicationFailure) wue.getCause()).getType());
        Assertions.assertEquals(
            "message='I was told I should fail', type='Failure', nonRetryable=false",
            wue.getCause().getMessage());
      }
      // issue an update that will succeed after `requestedSleep`
      updateId = UUID.randomUUID().toString();
      UpdateHandle<Integer> timeoutHandle =
          untypedStub.startUpdate(
              UpdateOptions.newBuilder(Integer.class)
                  .setUpdateName("update")
                  .setUpdateId(updateId)
                  .setFirstExecutionRunId(run.execution.getRunId())
                  .setWaitForStage(WorkflowUpdateStage.ACCEPTED)
                  .build(),
              true);
      // Expect to get a timeout exception
      try {
        timeoutHandle.getResultAsync(1, TimeUnit.SECONDS).get();
        Assertions.fail("unreachable");
      } catch (Exception e) {
        Assertions.assertTrue(e.getCause() instanceof WorkflowUpdateTimeoutOrCancelledException);
      }

      stub.finish();
      runner.requireNoUpdateRejectedEvents(run);
      return run;
    }
  }
}
