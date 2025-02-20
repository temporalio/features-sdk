<?php

declare(strict_types=1);

namespace Harness\Feature\ContinueAsNew\ContinueAsSame;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

\define('INPUT_DATA', 'InputData');
\define('MEMO_KEY', 'MemoKey');
\define('MEMO_VALUE', 'MemoValue');
\define('WORKFLOW_ID', 'PHP-ContinueAsNew-ContinueAsSame-TestID');

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('Workflow')]
    public function run(string $input)
    {
        if (!empty(Workflow::getInfo()->continuedExecutionRunId)) {
            return $input;
        }

        return yield Workflow::continueAsNew(
            'Workflow',
            args: [$input],
        );
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(
        #[Stub(
            type: 'Workflow',
            workflowId: WORKFLOW_ID,
            args: [INPUT_DATA],
            memo: [MEMO_KEY => MEMO_VALUE],
        )]
        WorkflowStubInterface $stub
    ): void {
        Assert::same($stub->getResult(), INPUT_DATA);
        # Workflow ID was not changed after continue as new
        Assert::same($stub->getExecution()->getID(), WORKFLOW_ID);
        # Memos do not change after continue as new
        $description = $stub->describe();
        Assert::same($description->info->memo->getValues(), [MEMO_KEY => MEMO_VALUE]);
    }
}
	