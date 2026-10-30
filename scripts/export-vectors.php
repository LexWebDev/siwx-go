<?php

function uses(mixed ...$arguments): object
{
    return new class
    {
        public function in(mixed ...$arguments): void {}
    };
}

require $argv[1];

$names = [
    'EVM_SIGNER', 'EVM_MESSAGE', 'EVM_RESOURCES_MESSAGE', 'EVM_SIG', 'EVM_RESOURCES_SIG', 'EVM_FOREIGN_SIG',
    'SOLANA_SIGNER', 'SOLANA_MESSAGE', 'SOLANA_SIG_BASE64', 'SOLANA_SIG_BASE58',
    'PHANTOM_SIGNER', 'PHANTOM_MESSAGE', 'PHANTOM_SIG',
];

$vectors = [];

foreach ($names as $name) {
    $vectors[$name] = constant($name);
}

echo json_encode($vectors, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES) . PHP_EOL;
