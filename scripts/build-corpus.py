import json
import sys
from pathlib import Path

vectors = json.loads(Path(sys.argv[1]).read_text())
evm = vectors['EVM_MESSAGE']
resources = vectors['EVM_RESOURCES_MESSAGE']
solana = vectors['SOLANA_MESSAGE']
phantom = vectors['PHANTOM_MESSAGE']
signer = vectors['EVM_SIGNER']
statement = 'Sign in with Ethereum to the application.'
recap = "I further authorize the stated URI to perform the following actions on my behalf: (1) 'request': 'personal_sign' for 'eip155'."

corpus = [evm, resources, solana, phantom, 'https://' + evm, 'http://' + phantom, '', ' ', 'garbage', 'garbage\n' + signer]

for field_value in [
    '2026-07-30T12:34:56Z', '2026-07-30T12:34:56.1Z', '2026-07-30T12:34:56.123456789Z', '2026-07-30T15:34:56+03:00',
    '2026-07-30T07:34:56-05:00', '2026-07-30t12:34:56z', '2026-07-30', '2026-07-30T12:34:56', 'now', 'tomorrow', '+1 day',
    '2026-02-31T12:34:56Z', '2026-13-01T12:34:56Z', '2026-07-30T25:00:00Z', '2026-07-30T12:60:00Z', '2026-07-30T12:34:60Z',
    '2026-07-30 12:34:56Z', '2026-07-30T12:34:56+0300', '2026-07-30T1:34:56Z', '2026-07-30T12:34:56,5Z', '0000-01-01T00:00:00Z',
]:
    corpus.append(evm.replace('2026-07-30T12:34:56.000Z', field_value, 1))

for chain in [
    '1', '137', '0', '007', '99999999999999999999', 'eip155:1', 'eip155:137', 'eip155:', ':1', 'eip155:!!!', 'mainnet',
    'solana:mainnet', 'solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp', 'solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpX', 'EIP155:1',
    'ab:1', 'abcdefghi:1', 'bip122:000000000019d6689c085ae165831e93', 'cosmos:cosmoshub-4', 'a:b:c', '-1', '1.5', '',
]:
    corpus.append(evm.replace('Chain ID: 1', 'Chain ID: ' + chain, 1))

for nonce in ['abc', '1234567', '12345678', 'abcd-1234-efgh', 'abcd 1234', 'Ключ12345678']:
    corpus.append(evm.replace('8Kc2fQ1pXm9Zr4Lt', nonce, 1))

for suffix in [
    '\nURI: https://dapp.expert', '\nNonce: zzzz9999zzzz', '\nInjected: value', '\n- https://dapp.expert', '\nResources:',
    '\nResources:\n- a\n- b', '\nResources:\n- a\nNonce: zzzz9999zzzz', '\nResources:\n\n- a', '\nResources:\n- a\nResources:\n- b',
    '\nRequest ID: req-42', '\nRequest ID: ', '\nExpiration Time: 2026-07-30T13:34:56.000Z', '\nNot Before: 2026-07-30T12:00:00.000Z',
    '\nExpiration Time: 2026-07-30T13:34:56.000Z\nNot Before: 2026-07-30T12:00:00.000Z\nRequest ID: x\nResources:\n- urn:recap:e30',
    '\n\n', '\n', ' ', '\t', '\x00', '\x0b', '\r\n', '\nVersion: 1', '\nIssued At: 2026-07-30T12:34:56.000Z', '\nChain ID: 1',
]:
    corpus.append(evm + suffix)

for statement_text in [
    '', 'Note: colon inside', 'URI: looks like a field', 'Line one\nLine two', 'Юникод в тексте', statement + ' ' + recap, recap,
    '   ', 'Resources:',
]:
    corpus.append(evm.replace(statement, statement_text, 1))
    corpus.append(resources.replace(statement, statement_text, 1))

for header_form in [
    'dapp.expert wants you to sign in with your Bitcoin Testnet account:', 'dapp.expert  wants you to sign in with your Ethereum account:',
    ' dapp.expert wants you to sign in with your Ethereum account:', 'dapp.expert wants you to sign in with your  account:',
    'ftp://dapp.expert wants you to sign in with your Ethereum account:', '1http://dapp.expert wants you to sign in with your Ethereum account:',
    'https:// wants you to sign in with your Ethereum account:', 'https://dapp.expert:8443 wants you to sign in with your Ethereum account:',
    'ДАПП.рф wants you to sign in with your Ethereum account:',
]:
    corpus.append(evm.replace('dapp.expert wants you to sign in with your Ethereum account:', header_form, 1))

corpus.append(evm.replace('\n', '\r\n'))
corpus.append(evm.replace('URI: https://dapp.expert', 'URI: '))
corpus.append(evm.replace('Version: 1', 'Version: '))
corpus.append(evm.replace('URI: https://dapp.expert', 'URI: https://dapp.expert\r\r'))
corpus.append(evm.replace('URI: https://dapp.expert', 'URI: \r\r'))
corpus.append(evm + '\rNonce: zzzz9999zzzz')
corpus.append('\n\n' + evm)
corpus.append(evm.replace(signer, '   ', 1))
corpus.append(evm.replace(signer, '  ' + signer + '  ', 1))
corpus.append(evm.split('\n')[0])
corpus.append('\n'.join(evm.split('\n')[:2]))

Path(sys.argv[2]).write_text(json.dumps(corpus, ensure_ascii=False))
print(len(corpus), 'messages')
