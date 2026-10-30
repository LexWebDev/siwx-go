<?php

require $argv[1];

use LexWebDev\Siwx\SiwxParser;

$corpus = json_decode(file_get_contents($argv[2]), true);
$results = [];
$parser = new SiwxParser;

foreach ($corpus as $raw) {
    try {
        $message = $parser->parse($raw);
        $results[] = [
            'ok' => true,
            'raw' => $message->raw,
            'scheme' => $message->scheme ?? '',
            'domain' => $message->domain,
            'network' => $message->network,
            'address' => $message->address,
            'statement' => $message->statement ?? '',
            'signedStatement' => $message->signedStatement ?? '',
            'uri' => $message->uri,
            'version' => $message->version,
            'namespace' => $message->namespace,
            'chainId' => $message->chainId,
            'nonce' => $message->nonce,
            'issuedAt' => $message->issuedAt->utc()->format('Y-m-d\TH:i:s'),
            'expiration' => $message->expirationTime?->utc()->format('Y-m-d\TH:i:s') ?? '',
            'notBefore' => $message->notBefore?->utc()->format('Y-m-d\TH:i:s') ?? '',
            'requestId' => $message->requestId ?? '',
            'resources' => $message->resources,
        ];
    } catch (LexWebDev\Siwx\Exceptions\SiwxException $exception) {
        $results[] = ['ok' => false, 'code' => $exception->code()];
    } catch (Throwable $exception) {
        $results[] = ['ok' => false, 'code' => 'CRASH ' . $exception::class];
    }
}

echo json_encode($results, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES | JSON_INVALID_UTF8_SUBSTITUTE);
