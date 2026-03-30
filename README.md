# resty

Минималистичный HTTP-фреймворк на Go для JSON API: типобезопасная регистрация эндпоинтов через дженерики, единый пайплайн валидации и middleware, встроенные метрики, трейсинг, логирование и HTML-страница для ручного тестирования API.

**Модуль:** `github.com/porebric/resty`  
**Лицензия:** Apache-2.0

## Требования

- Go **1.25+** (см. `go.mod`)

## Установка

```bash
go get github.com/porebric/resty
```

## Основные возможности

| Возможность | Описание |
|-------------|----------|
| **Роутинг** | [gorilla/mux](https://github.com/gorilla/mux) |
| **Эндпоинты** | `resty.Endpoint[R]` — один тип запроса `R`, парсер из `*http.Request`, обработчик возвращает `responses.Response` и HTTP-код |
| **Валидация** | Обязательный `Request.Validate()` в цепочке middleware |
| **Документация** | Метаданные для OpenAPI-подобного описания; страница **`GET /api/swagger.html`** с «попробовать запрос» |
| **Наблюдаемость** | Prometheus **`/metrics`**, трейсы через [porebric/tracer](https://github.com/porebric/tracer), логи [porebric/logger](https://github.com/porebric/logger) |
| **Отладка** | `net/http/pprof` на **`/debug/pprof/*`** |
| **WebSocket** | Опциональный `ws.Hub`, эндпоинт **`/ws`** при вызове `RunServer` с непустым hub |
| **CORS** | Через `Router.SetCors` и [rs/cors](https://github.com/rs/cors) |
| **Конфигурация** | Порт и таймаут graceful shutdown через [porebric/configs](https://github.com/porebric/configs) |

## Конфигурация сервера

Через `configs` в контексте (ключи задаются в вашем приложении):

| Ключ | Назначение | Значение по умолчанию |
|------|------------|------------------------|
| `server_port` | Порт HTTP | `8080` |
| `close_timeout` | Таймаут на shutdown-хуки | `3s` |

## Быстрый старт

### 1. Инициализация кодов ошибок

Перед обработкой запросов вызовите `errors.Init` (при необходимости передайте свою карту `map[int32]CustomError`):

```go
import "github.com/porebric/resty/errors"

func main() {
    errors.Init(nil) // или errors.Init(map[int32]errors.CustomError{...})
    // ...
}
```

### 2. Роутер и логгер

```go
import (
    "context"
    "github.com/porebric/logger"
    "github.com/porebric/resty"
)

logFn := func() *logger.Logger { return logger.FromContext(ctx) } // пример; подставьте свой способ получения логгера
r := resty.NewRouter(logFn, nil) // второй аргумент — *ws.Hub или nil
```

### 3. Тип запроса

Реализуйте интерфейс `requests.Request`:

```go
type MyRequest struct {
    // поля с json-тегами для примеров в swagger UI
}

func (MyRequest) Path() (string, bool) { return "/api/v1/hello", false }
func (MyRequest) Methods() []string   { return []string{"POST"} }
func (MyRequest) String() string      { return "hello" }

func (MyRequest) Validate() (bool, string, string) {
    return true, "", ""
}
```

Опционально для документации и UI:

- `OpenAPIDoc` — `OpenAPISummary()`, `OpenAPIDescription()`
- `OpenAPIBody` — пример тела запроса (JSON-строка)
- `OpenAPIResponses` — `map[int]string` примеров ответов по коду
- Для **GET** query-параметры в UI выводятся из **json-тегов** полей структуры (имена приводятся к snake_case)

### 4. Парсер запроса и обработчик

```go
import (
    "context"
    "net/http"
    "github.com/porebric/resty"
    "github.com/porebric/resty/responses"
)

initMy := func(ctx context.Context, r *http.Request) (context.Context, MyRequest, error) {
    // распарсить тело/заголовки в MyRequest
    return ctx, MyRequest{}, nil
}

action := func(ctx context.Context, req MyRequest) (responses.Response, int) {
    return &responses.SuccessResponse{Success: true, Message: "ok"}, http.StatusOK
}

resty.Endpoint(r, initMy, action)
// опционально: дополнительные middleware — resty.Endpoint(r, initMy, action, MyAuthMiddleware)
```

### 5. Запуск

```go
resty.RunServer(ctx, r,
    func(ctx context.Context) error { /* закрыть БД и т.д. */ return nil },
)
```

При переданном WebSocket-hub на `/ws` поднимается обработчик; в фоне запускается `hub.Run()`.

## Middleware

Цепочка всегда начинается с `middleware.RequestValidate` (вызов `Validate()` у запроса). Дальше можно добавить свои реализации `middleware.Middleware`: `Execute(ctx, req) → (ctx, code, msg)`, при `code != ErrorNoError` клиент получит ответ из `errors.GetCustomError`.

Фабрики передаются в `Endpoint` как `...func() middleware.Middleware`.

## Ответы

- `responses.SuccessResponse` — `{ "success", "message" }`
- `responses.ErrorResponse` — `{ "code", "message" }` (используется фреймворком и `errors`)

Реализуйте `responses.Response` для кастомных тел.

## Документация API

- **`GET /api/swagger.html`** — список эндпоинтов, зарегистрированных через `Endpoint`, с формой отправки запроса.
- **`Router.RegisterDoc`** — только метаданные для вашей генерации спецификации (не показываются на swagger.html).
- **`Router.GetDocRoutes()`** / **`GetAppDocRoutes()`** — срезы `RouteDoc` для сборки OpenAPI JSON/YAML в вашем сервисе.

## Встроенные пути

| Путь | Назначение |
|------|------------|
| `/metrics` | Prometheus |
| `/debug/pprof/...` | Профилирование |
| `/api/swagger.html` | Интерактивная документация приложения |
| `/ws` | WebSocket (если hub не nil) |

## Зависимости

Основные прямые зависимости: `gorilla/mux`, `gorilla/websocket`, `prometheus/client_golang`, `rs/cors`, `google/uuid`, а также модули **porebric**: `configs`, `logger`, `tracer`.

## Пример в репозитории

Каталог [`example/`](example/) — заготовка под демо-сервер (можно развить до полного примера).

---

Вопросы и PR — через [Issues](https://github.com/porebric/resty/issues) репозитория.
