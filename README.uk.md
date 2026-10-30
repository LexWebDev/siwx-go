# siwx-go

[English](README.md) | **Українська**

[![go reference](https://pkg.go.dev/badge/github.com/LexWebDev/siwx-go.svg)](https://pkg.go.dev/github.com/LexWebDev/siwx-go)
[![go](https://img.shields.io/github/go-mod/go-version/LexWebDev/siwx-go?logo=go&logoColor=white&label=go)](go.mod)
[![tests](https://img.shields.io/github/actions/workflow/status/LexWebDev/siwx-go/tests.yml?branch=main&logo=github&label=tests)](https://github.com/LexWebDev/siwx-go/actions/workflows/tests.yml?query=branch%3Amain)
[![license](https://img.shields.io/github/license/LexWebDev/siwx-go?color=blue)](LICENSE)

Серверна перевірка Sign-In With X для Go. Перевіряє підписи гаманців над повідомленнями
[EIP-4361](https://eips.ethereum.org/EIPS/eip-4361) для акаунтів **EVM** та **Solana** і коректно
працює з WalletConnect One-Click Auth.

Go-аналог пакета [lexwebdev/laravel-siwx](https://github.com/LexWebDev/laravel-siwx). Вони мають
спільні тест-вектори й коди помилок, тож один фронтенд може працювати з будь-яким із двох бекендів.

## Навіщо це потрібно

Наївний верифікатор SIWE працює на десктопі в ідеальному сценарії та ламається, щойно з'являється
справжній гаманець:

- **Заголовок не фіксований.** AppKit будує перший рядок як
  `${domain} wants you to sign in with your ${networkName} account:`, де `networkName`
  береться з конфігурації мережі: `Solana`, `Polygon`, будь-що. Парсер, який шукає слово
  `Ethereum`, відхиляє кожен вхід не з EVM.
- **`Chain ID` — не число.** AppKit передає ідентифікатор CAIP-2 (`eip155:1`,
  `solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp`), а шлях One-Click Auth будує повідомлення через
  `formatAuthMessage` від WalletConnect і видає звичайне ціле число, як у EIP-4361. Обидва варіанти
  приходять на той самий ендпоінт. Цей модуль приймає обидва й нормалізує їх до CAIP-2.
- **One-Click Auth додає секцію `Resources:`** з можливістю `urn:recap:`. Вона є частиною
  підписаних байтів, тому її треба зберегти для перевірки підпису й ігнорувати під час розбору полів.
- **One-Click Auth переписує statement.** Згідно з [ERC-5573](https://eips.ethereum.org/EIPS/eip-5573)
  гаманець дописує `I further authorize the stated URI to perform the following actions on my
  behalf: (1) 'request': 'personal_sign' …` до statement, який запросив застосунок, а список методів
  у різних гаманців різний. Застосунок, що порівнює statement із запитаним текстом, відхиляє кожен
  мобільний вхід. Цей модуль повертає власний statement застосунку в `Statement`, а повний підписаний
  рядок зберігає в `SignedStatement`.
- **Регістр адреси залежить від мережі.** Переведення адреси Solana в base58 у нижній регістр дає
  іншу, неіснуючу адресу.

Кожен із цих випадків покрито тестом на реальному векторі підпису.

## Підтримувані мережі

| Простір імен CAIP-2 | Пакет | Мережі | Схема підпису |
|---|---|---|---|
| `eip155` | `evm` | усі мережі EVM | secp256k1 + keccak256, EIP-191, опційно EIP-1271 |
| `solana` | `solana` | Solana mainnet/devnet | ed25519, підписи в base58 та base64 |

Кожна мережа — окремий пакет, і ввімкнено лише те, що ви передали в `Config.Verifiers`. Застосунок
лише для Solana ніколи не компілює залежності EVM. Власний `SignatureVerifier` підключається так само,
як вбудовані, без змін у ядрі:

```go
verifier, err := siwx.New(siwx.Config{
	Domains:   []string{"app.example"},
	Nonces:    store,
	Verifiers: []siwx.SignatureVerifier{evm.New(nil), solana.New(), myBip122Verifier},
})
```

## Встановлення

```bash
go get github.com/LexWebDev/siwx-go
```

## Конфігурація

`Domains` на практиці **обов'язкове** поле. Якщо список порожній, кожна перевірка завершується
помилкою `siwx_invalid_domain`. Це зроблено навмисно: порожній список безпечніший за відкритий, а
прив'язка до домену якраз і не дає підпису, отриманому на іншому сайті, спрацювати на вашому.

| Поле | Призначення |
|---|---|
| `Domains` | список дозволених доменів |
| `Nonces` | **обов'язкове**, `NonceStore` |
| `Verifiers` | **обов'язкове**, увімкнені мережі |
| `ClockSkew` | допуск на годинник гаманця, що поспішає, типово п'ять хвилин; `siwx.NoClockSkew` вимикає його |
| `Now` | замінює `time.Now`, корисно в тестах |

Вказуйте кожен домен **один раз**, точно так, як гаманець надішле його в рядку `domain`, зокрема з
портом, якщо застосунок працює на порту, наприклад `localhost:3000` під час розробки.

Поле `URI` не звіряється зі списком дозволених окремо. Натомість його authority (хост і порт, якщо
він є) має збігатися з рядком `domain`, як того вимагає EIP-4361 для коректного повідомлення. Це
суворіше, ніж перевіряти обидва значення за списком: за двох дозволених хостів повідомлення могло б
назвати один домен, а в URI вказати на інший.

## Використання

`siwx.NonceHandler` відповідає на запити `GET` тілом `{"nonce": "..."}`. Nonce одноразові й спливають
після TTL сховища.

Обробник не має обмеження частоти запитів, а кожен запит зберігає nonce. **Поставте перед ним лімітер.**

Маршрутів автентифікації навмисно немає: вони залежать від вашої моделі користувача та стратегії
токенів. Мінімальний сервер:

```go
store := memorynonce.New(10 * time.Minute)

verifier, err := siwx.New(siwx.Config{
	Domains:   []string{"app.example"},
	Nonces:    store,
	Verifiers: []siwx.SignatureVerifier{evm.New(nil), solana.New()},
})
if err != nil {
	log.Fatal(err)
}

http.Handle("GET /siwx/nonce", siwx.NonceHandler(store))

http.HandleFunc("POST /api/auth", func(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Message   string `json:"message"`
		Signature string `json:"signature"`
	}

	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 8192)).Decode(&body); err != nil {
		http.Error(writer, "invalid payload", http.StatusBadRequest)
		return
	}

	session, err := verifier.Verify(request.Context(), body.Message, body.Signature)

	var failure *siwx.Error
	if errors.As(err, &failure) {
		http.Error(writer, failure.Code, http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(writer, "internal error", http.StatusInternalServerError)
		return
	}

	issueYourOwnSession(writer, session.Namespace, session.Address)
})
```

У разі успіху `Verify` повертає `*siwx.Session`:

```go
session.Address   // normalised per chain
session.Namespace // "eip155" or "solana"
session.ChainID   // always CAIP-2, e.g. "eip155:137"
session.Domain
session.IssuedAt  // time.Time
session.Message   // the parsed *siwx.Message
```

`Message` містить усі розібрані поля. Ось ті, які варто знати, коли ви перевіряєте намір користувача:

```go
message.Statement       // what your app asked the wallet to sign, recap sentence removed
message.SignedStatement // the statement line exactly as it appears in the signed message
message.Resources       // entries of the Resources section, e.g. ["urn:recap:…"]
message.Scheme          // optional scheme of the header line, e.g. "https", or empty
message.RequestID       // the Request ID field, or empty
```

Порівнюйте із запитаним текстом `Statement`, а не `SignedStatement`. На шляху One-Click Auth вони
різняться, і ця різниця залежить від гаманця.

### Сховища nonce

| Сховище | Для чого |
|---|---|
| `memorynonce.New` | один процес |
| `redisnonce.New` | кілька процесів; потрібен Redis 6.2+ для `GETDEL` |

Власний `NonceStore` має робити `Consume` атомарним. Читання з наступним видаленням дозволяє двом
паралельним запитам з тим самим підписаним повідомленням пройти обом.

### Зберігання адреси

`session.Address` нормалізується **залежно від мережі**: адреси EVM переводяться в нижній регістр,
а адреси Solana в base58 повертаються байт у байт, бо base58 чутливий до регістру.

Ніколи не зберігайте лише адресу. Той самий рядок може бути дійсним акаунтом у кількох просторах
імен, тому унікальний індекс має охоплювати пару `(namespace, address)`.

## Коди помилок

Кожна відмова обгортає `*siwx.Error`, який можна перевірити через `errors.Is`. Надсилайте клієнту
`failure.Code`, а не `err.Error()`. Якщо виклик EIP-1271 зазнає збою, помилка обгортає і
`ErrInvalidSignature`, і причину, наприклад `evm.ErrRPCTimeout`, тож вхід відхилено, а збій можна
записати в лог. Будь-яка інша помилка, наприклад збій Redis, є інфраструктурною й передається без змін.

| Код | Сентинел | Значення |
|---|---|---|
| `siwx_invalid_message` | `ErrInvalidMessage` | повідомлення порушує граматику EIP-4361 (дубльовані чи невідомі поля, багаторядковий statement, час поза форматом RFC 3339, `Chain ID` поза CAIP-2), неправильна версія або не пройдено перевірку часу |
| `siwx_invalid_domain` | `ErrInvalidDomain` | `domain` або хост `URI` не входить до списку дозволених, або список порожній |
| `siwx_invalid_nonce` | `ErrInvalidNonce` | nonce ніколи не видавався, уже використаний або сплив |
| `siwx_invalid_signature` | `ErrInvalidSignature` | підпис не належить заявленій адресі |
| `siwx_unsupported_namespace` | `ErrUnsupportedNamespace` | для цього простору імен не ввімкнено верифікатор |

Nonce гаситься **після** успішної перевірки підпису, тому неправильним підписом не можна спалити
чужий nonce.

## Фронтенд

Використовуйте `SIWXConfig` з `@reown/appkit`: окремий пакет `@reown/appkit-siwx` встановлювати не
потрібно.

```ts
import { createAppKit } from '@reown/appkit'

createAppKit({
  // ...
  siwx: {
    async getNonce() {
      const { nonce } = await fetch('/siwx/nonce').then((r) => r.json())
      return nonce
    },
    async addSession({ message, signature }) {
      await fetch('/api/auth', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message, signature }),
      })
    },
    // getSessions, revokeSession, setSessions per your app
  },
})
```

На шляху One-Click Auth `createMessage` викликається без адреси, тож вивести nonce з гаманця
неможливо: спершу отримайте його з `GET /siwx/nonce`.

### Phantom відмовляється підписувати chain id із genesis-хешем у Solana

AppKit передає в `createMessage` ідентифікатор CAIP-2 `solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp`.
Розширення Phantom, за перевіркою від 2026-10-30, не підписує повідомлення з ним: гаманець повертає
помилку "The app's signature request cannot be shown due to invalid formatting", AppKit показує
"Error signing message", і до вашого бекенду нічого не доходить. Натомість пишіть у рядку `Chain ID`
значення `solana:mainnet`:

```js
const signedChainId = chainId === 'solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp' ? 'solana:mainnet' : chainId
```

Цей модуль приймає обидві форми й повертає значення так, як його підписано, тому в `session.ChainID`
може бути будь-яка з них. Якщо ваша логіка залежить від мережі, порівнюйте з обома.

### One-Click Auth спрацьовує лише в застосунку з одним простором імен

AppKit пробує One-Click Auth (`wc_sessionAuthenticate`, який повертає CACAO і повідомлення, зібране
WalletConnect, а не вашим `createMessage`) лише тоді, коли всі зареєстровані мережі належать
**рівно одному** простору імен, і це `eip155`.

Отже, застосунок, який реєструє мережі EVM **і** Solana в одному екземплярі AppKit, ніколи не отримає
CACAO: кожен вхід відкочується до звичайного `personal_sign` повідомлення, зібраного вашим кодом.
Це варто знати, перш ніж шукати помилку в обробці `Resources`: секції `Resources` просто не буде.

Цей модуль перевіряє обидва варіанти, тож налаштовувати нічого не треба.

Для тестування з цього випливають дві речі. Гаманець без підтримки `wc_sessionAuthenticate` теж мовчки
відкочується, тому відсутність CACAO сама по собі не каже, яка сторона відмовила. А запит надсилається
під час підключення, а не під час підпису: якщо запит на підпис з'являється вже після встановлення
сесії, One-Click Auth не відбувся.

## Гаманці на смартконтрактах (EIP-1271)

Типово вимкнено. Передайте перевірник в `evm.New`, і підпис, який не пройшов відновлення адреси,
отримає другий шанс через `eth_call` до `isValidSignature` на заявленій адресі:

```go
contracts := evm.NewRPCChecker("", map[string]string{
	"eip155:1":    mainnetURL,
	"eip155:8453": baseURL,
})

verifier, err := siwx.New(siwx.Config{
	Domains:   domains,
	Nonces:    store,
	Verifiers: []siwx.SignatureVerifier{evm.New(contracts), solana.New()},
})
```

Коли мапу задано, мережа поза нею не проходить перевірку. Єдиний URL першим аргументом натомість
обслуговує всі мережі, тому підпис перевіряється в мережі цього ендпоінта, яким би не був `Chain ID`
у повідомленні. Використовуйте це, лише якщо приймаєте одну мережу.

**Адреса контракту належить одній мережі, а не одному власнику.** Ту саму адресу в двох мережах можуть
контролювати різні сторони. Якщо у мапі більше однієї мережі, ідентифікуйте акаунти контрактних
гаманців за `(session.ChainID, session.Address)`, а не лише за адресою, або залиште тільки одну мережу,
якій довіряєте. З єдиним URL сесія показує мережу, заявлену в повідомленні, а не ту, де відбулася
перевірка, тому в цьому режимі вважайте `session.ChainID` неперевіреним.

Для входів з EOA це нічого не коштує: виклик RPC відбувається лише після невдалого відновлення адреси
і лише для ще чинного nonce, тож запит із вигаданим nonce не витрачає вашу квоту RPC. Контракт, який
робить revert, означає звичайну відмову. Це єдине місце, де модуль звертається до мережі, а його
помилки є фіксованими рядками: до них не потрапляють ні адреса ендпоінта, яка часто містить API-ключ,
ні текст від вузла. Замінити реалізацію можна, передавши власний `evm.ContractChecker`.

## Сумісність

| | |
|---|---|
| Go | 1.26+, CI проганяє 1.26 та 1.27 з race detector |
| Фреймворки | не потрібні, обробник є звичайним `http.Handler` |
| Redis | 6.2+ для `redisnonce` |

## Перевірка на справжніх гаманцях

`examples/stand` — невеликий сервер із маршрутами `GET /siwx/nonce` та `POST /api/siwx/verify`. Він
читає змінні, перелічені в `.env.example`, із середовища.

```bash
go run ./examples/stand
```

## Участь у розробці

Звіти про помилки та pull request вітаються. У [CONTRIBUTING.md](CONTRIBUTING.md) описано, як запускати
тести, і правила для коду перевірки. Прочитайте його, перш ніж змінювати верифікатор, бо деякі правила
неочевидні. Участь регулюється [Кодексом поведінки](CODE_OF_CONDUCT.md).

## Безпека

Див. [SECURITY.md](SECURITY.md). Будь ласка, не повідомляйте про вразливості в публічних issue.

## Ліцензія

MIT. Див. [LICENSE](LICENSE).
