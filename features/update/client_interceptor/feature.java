package update.client_interceptor;

import io.temporal.client.UpdateHandle;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.client.UpdateHandle;
import io.temporal.common.interceptors.WorkflowClientCallsInterceptor;
import io.temporal.common.interceptors.WorkflowClientCallsInterceptorBase;
import io.temporal.common.interceptors.WorkflowClientInterceptorBase;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.*;
import org.junit.jupiter.api.Assertions;

@WorkflowInterface
public interface feature extends Feature {
  @UpdateMethod
  int update(int i);

  @SignalMethod
  void finish();

  @WorkflowMethod
  int workflow();

  class Impl implements feature {

    private boolean doFinish = false;
    private int counter = 0;

    @Override
    public int workflow() {

      Workflow.await(() -> this.doFinish);
      return counter;
    }

    @Override
    public int update(int i) {
      counter += i;
      return counter;
    }

    @Override
    public void finish() {
      this.doFinish = true;
    }

    @Override
    public void workflowClientOptions(WorkflowClientOptions.Builder builder) {
      builder.setInterceptors(
          new WorkflowClientInterceptorBase() {
            @Override
            public WorkflowClientCallsInterceptor workflowClientCallsInterceptor(
                WorkflowClientCallsInterceptor next) {
              return new WorkflowClientCallsInterceptorBase(next) {
                @Override
                public <R> UpdateHandle<R> startUpdate(StartUpdateInput<R> input) {
                  if (input.getUpdateName() == "update") {
                    input.getArguments()[0] = ((int) input.getArguments()[0]) + 1;
                  }
                  return super.startUpdate(input);
                }
              };
            }
          });
    }

    @Override
    public Run execute(Runner runner) {
      runner.skipIfUpdateNotSupported();

      var run = runner.executeSingleParameterlessWorkflow();
      var stub = runner.client.newWorkflowStub(feature.class, run.execution.getWorkflowId());

      int updateResult = stub.update(5);
      Assertions.assertEquals(6, updateResult);

      updateResult = stub.update(3);
      Assertions.assertEquals(10, updateResult);

      stub.finish();
      return run;
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      Assertions.assertEquals(10, runner.waitForRunResult(run));
    }
  }
}
