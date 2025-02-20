<?php

declare(strict_types=1);

use Harness\Exception\SkipTest;
use Harness\Feature\WorkflowStubInjector;
use Harness\Runtime\Feature;
use Harness\Runtime\Runner;
use Harness\Runtime\State;
use Harness\RuntimeBuilder;
use Harness\Support;
use Psr\Container\ContainerInterface;
use Spiral\Core\Attribute\Proxy;
use Spiral\Core\Container;
use Spiral\Core\Scope;
use Spiral\Goridge\RPC\RPC;
use Spiral\Goridge\RPC\RPCInterface;
use Spiral\RoadRunner\KeyValue\Factory;
use Spiral\RoadRunner\KeyValue\StorageInterface;
use Temporal\Client\ClientOptions;
use Temporal\Client\GRPC\ServiceClient;
use Temporal\Client\GRPC\ServiceClientInterface;
use Temporal\Client\ScheduleClient;
use Temporal\Client\ScheduleClientInterface;
use Temporal\Client\WorkflowClient;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowStubInterface;
use Temporal\DataConverter\DataConverter;
use Temporal\DataConverter\DataConverterInterface;

require_once __DIR__ . '/src/RuntimeBuilder.php';
RuntimeBuilder::init();

$runtime = RuntimeBuilder::createState($argv, \getcwd());

$runner = new Runner($runtime);

// Run RoadRunner server if workflows or activities are defined
if (\iterator_to_array($runtime->workflows(), false) !== [] || \iterator_to_array($runtime->activities(), false) !== []) {
    $runner->start();
}

// Prepare and run checks

// Prepare services to be injected

$serviceClient = $runtime->command->tlsKey === null && $runtime->command->tlsCert === null
    ? ServiceClient::create($runtime->address)
    : ServiceClient::createSSL(
        $runtime->address,
        clientKey: $runtime->command->tlsKey,
        clientPem: $runtime->command->tlsCert,
    );
echo "Connecting to Temporal service at {$runtime->address}... ";
try {
    $serviceClient->getConnection()->connect(5);
    echo "\e[1;32mOK\e[0m\n";
} catch (\Throwable $e) {
    echo "\e[1;31mFAILED\e[0m\n";
    Support::echoException($e);
    return;
}

// TODO if authKey is set
// $serviceClient->withAuthKey($authKey)

$converter = DataConverter::createDefault();

$workflowClient = WorkflowClient::create(
    serviceClient: $serviceClient,
    options: (new ClientOptions())->withNamespace($runtime->namespace),
    converter: $converter,
)->withTimeout(10);

$scheduleClient = ScheduleClient::create(
    serviceClient: $serviceClient,
    options: (new ClientOptions())->withNamespace($runtime->namespace),
    converter: $converter,
)->withTimeout(10);

$container = new Spiral\Core\Container();
$container->bindSingleton(State::class, $runtime);
$container->bindSingleton(Runner::class, $runner);
$container->bindSingleton(ServiceClientInterface::class, $serviceClient);
$container->bindSingleton(WorkflowClientInterface::class, $workflowClient);
$container->bindSingleton(ScheduleClientInterface::class, $scheduleClient);
$container->bindInjector(WorkflowStubInterface::class, WorkflowStubInjector::class);
$container->bindSingleton(DataConverterInterface::class, $converter);
$container->bind(RPCInterface::class, static fn() => RPC::create('tcp://127.0.0.1:6001'));
$container->bind(
    StorageInterface::class,
    fn (#[Proxy] ContainerInterface $c): StorageInterface => $c->get(Factory::class)->select('harness'),
);

// Run checks
$errors = 0;
foreach ($runtime->checks() as $feature => $definition) {
    try {
        $container->runScope(
            new Scope(name: 'feature',bindings: [
                Feature::class => $feature,
            ]),
            static function (Container $container) use ($definition) {
                // todo modify services based on feature requirements
                [$class, $method] = $definition;
                $container->bindSingleton($class, $class);
                echo "Running check \e[1;36m{$class}::{$method}\e[0m ";
                $container->invoke($definition);
                echo "\e[1;32mSUCCESS\e[0m\n";
            },
        );
    } catch (SkipTest $e) {
        echo "\e[1;33mSKIPPED\e[0m\n";
        echo "\e[35m{$e->reason}\e[0m\n";
    } catch (\Throwable $e) {
        echo "\e[1;31mFAILED\e[0m\n";

        \trap($e);
        ++$errors;
        Support::echoException($e);
        echo "\n";
    } finally {
        $runner->start();
    }
}

exit($errors === 0 ? 0 : 1);
