# API Gateway

## Описание
Go‑прокси на базе Gin объединяет auth-service (gRPC), llm-script-service и video-service (HTTP). Он отвечает за аутентификацию пользователей, выдачу JWT, проксирование запросов к сценариям и запуск задач генерации видео. Все внешние клиенты (веб/мобайл) общаются только с Gateway.

## Основные возможности
- `/api/auth/*` — регистрация, логин, обновление/логаут токенов, получение профиля и проверки роли.
- `/api/scripts` — защищённый прокси к llm-script-service.
- `/api/videos`, `/api/ideas/expand` — защищённый прокси к video-service (включая каталоги `/voices` и `/music`, работу с медиа и стрим статусов).
- `/healthz` — проверочный эндпоинт для оркестраторов.

## Технологии
- Go 1.21+, Gin, gRPC (auth).
- JWT (подписывается `APP_SECRET`).
- Клиенты на httpx (scripts/videos), автоматическое чтение `.env`.


## Установка и запуск
```bash
cd api-gateway
go run ./cmd/main.go --config=./config/local.yaml
# или
APP_SECRET=... CONFIG_PATH=./config/dev.yaml go run ./cmd/main.go
```

## Конфигурация
`config/*.yaml`:
- `env`, `http.host`, `http.port`, таймауты.
- `auth_grpc (address, timeout)` — адрес auth-service.
- `script_service` и `video_service` — базовые URL и таймауты.
Переменные можно переопределить через `.env` (APP_SECRET, JWT TTL и т.д.).
