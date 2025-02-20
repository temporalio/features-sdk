<?php

declare(strict_types=1);

namespace Harness\Feature;

use Harness\Attribute\Stub;
use Harness\Exception\SkipTest;
use Harness\Runtime\Feature;
use Psr\Container\ContainerInterface;
use Spiral\Core\Attribute\Proxy;
use Spiral\Core\Container\InjectorInterface;
use Temporal\Client\WorkflowOptions;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Common\IdReusePolicy;

/**
 * @implements InjectorInterface<WorkflowStubInterface>
 */
final class WorkflowStubInjector implements InjectorInterface
{
    public function __construct(
        #[Proxy] private readonly ContainerInterface $container,
        private readonly ClientFactory $clientFactory,
    ) {
    }

    public function createInjection(
        \ReflectionClass $class,
        \ReflectionParameter|null|string $context = null,
    ): WorkflowStubInterface {
        if (!$context instanceof \ReflectionParameter) {
            throw new \InvalidArgumentException('Context is not clear.');
        }

        /** @var Stub|null $attribute */
        $attribute = ($context->getAttributes(Stub::class)[0] ?? null)?->newInstance();
        if ($attribute === null) {
            throw new \InvalidArgumentException(\sprintf('Attribute %s is not found.', Stub::class));
        }

        $client = $this->clientFactory->workflowClient($context);

        if ($attribute->eagerStart) {
            // If the server does not support eager start, skip the test
            $client->getServiceClient()->getServerCapabilities()->eagerWorkflowStart or throw new SkipTest(
                'Eager workflow start is not supported by the server.'
            );
        }

        /** @var Feature $feature */
        $feature = $this->container->get(Feature::class);
        $options = WorkflowOptions::new()
            ->withTaskQueue($feature->taskQueue)
            ->withWorkflowIdReusePolicy(IdReusePolicy::AllowDuplicate)
            ->withEagerStart($attribute->eagerStart);

        $attribute->workflowId === null or $options = $options->withWorkflowId($attribute->workflowId);
        $attribute->memo === [] or $options = $options->withMemo($attribute->memo);

        $stub = $client->newUntypedWorkflowStub($attribute->type, $options);
        $client->start($stub, ...$attribute->args);

        return $stub;
    }
}
