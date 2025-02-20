<?php

declare(strict_types=1);

namespace Harness\Feature\Schedule\Basic;

use Carbon\CarbonImmutable;
use Carbon\CarbonInterval;
use Harness\Attribute\Check;
use Harness\Exception\SkipTest;
use Harness\Runtime\Feature;
use Harness\Runtime\State;
use Ramsey\Uuid\Uuid;
use Temporal\Client\Schedule\Action\StartWorkflowAction;
use Temporal\Client\Schedule\Policy\ScheduleOverlapPolicy;
use Temporal\Client\Schedule\Policy\SchedulePolicies;
use Temporal\Client\Schedule\Schedule;
use Temporal\Client\Schedule\ScheduleOptions;
use Temporal\Client\Schedule\Spec\ScheduleSpec;
use Temporal\Client\ScheduleClientInterface;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('Workflow')]
    public function run(string $arg)
    {
        return $arg;
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(
        ScheduleClientInterface $client,
        WorkflowClientInterface $wfClient,
        Feature $feature,
        State $runtime,
    ): void {
        $workflowId = Uuid::uuid4()->toString();
        $scheduleId = Uuid::uuid4()->toString();
        $interval = CarbonInterval::seconds(2);

        $handle = $client->createSchedule(
            schedule: Schedule::new()
                ->withAction(
                    StartWorkflowAction::new('Workflow')
                        ->withWorkflowId($workflowId)
                        ->withTaskQueue($feature->taskQueue)
                        ->withInput(['arg1'])
                )->withSpec(
                    ScheduleSpec::new()
                        ->withIntervalList($interval)
                )->withPolicies(
                    SchedulePolicies::new()
                        ->withOverlapPolicy(ScheduleOverlapPolicy::BufferOne)
                ),
            options: ScheduleOptions::new()
                ->withNamespace($runtime->namespace),
            scheduleId: $scheduleId,
        );
        try {
            $deadline = CarbonImmutable::now()->add($interval)->add($interval);

            // Confirm simple describe
            $description = $handle->describe();
            Assert::same($handle->getID(), $scheduleId);
            /** @var StartWorkflowAction $action */
            $action = $description->schedule->action;
            Assert::isInstanceOf($action, StartWorkflowAction::class);
            Assert::same($action->workflowId, $workflowId);

            // Confirm simple list
            $found = false;
            $findDeadline = \microtime(true) + 10;
            find:
            foreach ($client->listSchedules() as $schedule) {
                if ($schedule->scheduleId === $scheduleId) {
                    $found = true;
                    break;
                }
            }
            if (!$found and \microtime(true) < $findDeadline) {
                 goto find;
            }

            $found or throw new \Exception('Schedule not found');

            // Wait for first completion
            while ($handle->describe()->info->numActions < 1) {
                CarbonImmutable::now() < $deadline or throw new \Exception('Workflow did not execute');
                \usleep(100_000);
            }
            $handle->pause('Waiting for changes');

            // Check result
            $lastActions = $handle->describe()->info->recentActions;
            $lastAction = $lastActions[\array_key_last($lastActions)];
            $result = $wfClient->newUntypedRunningWorkflowStub(
                $lastAction->startWorkflowResult->getID(),
                $lastAction->startWorkflowResult->getRunID(),
                workflowType: 'Workflow'
            )->getResult();
            Assert::same($result, 'arg1');

            // Update and change arg
            $handle->update(
                $description->schedule->withAction(
                    $action->withInput(['arg2'])
                ),
            );
            $numActions = $handle->describe()->info->numActions;
            $handle->unpause('Run again');

            // Wait for second completion
            $deadline = CarbonImmutable::now()->add($interval)->add($interval);
            while ($handle->describe()->info->numActions <= $numActions) {
                CarbonImmutable::now() < $deadline or throw new \Exception('Workflow did not execute');
                \usleep(100_000);
            }

            // Check result 2
            $lastActions = $handle->describe()->info->recentActions;
            $lastAction = $lastActions[\array_key_last($lastActions)];
            $result = $wfClient->newUntypedRunningWorkflowStub(
                $lastAction->startWorkflowResult->getID(),
                $lastAction->startWorkflowResult->getRunID(),
                workflowType: 'Workflow'
            )->getResult();
            Assert::same($result, 'arg2');
        } finally {
            $handle->delete();
        }
    }
}
