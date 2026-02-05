# ImageProcessor — очередь фоновой обработки изображений

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://golang.org)
[![Kafka](https://img.shields.io/badge/Apache_Kafka-3.4+-231F20?logo=apache-kafka)](https://kafka.apache.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-4169E1?logo=postgresql)](https://www.postgresql.org)
[![MinIO](https://img.shields.io/badge/MinIO-Latest-2683C9?logo=minio)](https://min.io)

Сервис для асинхронной обработки изображений через очередь сообщений (Apache Kafka). Принимает изображения, ставит задачу в очередь и обрабатывает в фоновом режиме: ресайз, генерация миниатюр, добавление водяных знаков.

## Быстрый старт

```bash
# 1. Клонировать репозиторий
git clone https://github.com/sj-shoff/ImageProcessor.git
cd image-processor

# 2. Создать .env файл из шаблона
cp .env.example .env

# 3. Запустить все сервисы
make docker-up

# 4. Применить миграции БД
make migrate-up
```

Откройте в браузере: [http://localhost:8034](http://localhost:8034)

## Команды Makefile

>`make build` - Сборка бинарных файлов (`image-processor`, `worker`) 
>`make run` - Запуск HTTP-сервера локально 
>`make run-worker` - Запуск воркера обработки изображений локально 
>`make migrate` - Применение миграций БД через `goose` 
>`make migrate-down` - Откат последней миграции 
>`make kafka-init` - Создание топиков Kafka (`image-processing`, `image-processed`) 
>`make docker-up` - Сборка и запуск всего стека через Docker Compose 
>`make docker-down` - Остановка контейнеров 

## Требования

- **Go** 1.24+
- **Docker** и **Docker Compose**

## Конфигурация

Конфигурация загружается из `.env` файла

## HTTP API

### `POST /api/images/upload`
Загрузка изображения на обработку.

**Параметры формы:**
- `file` (required) — изображение (до 32 МБ)
- `thumbnail` (optional, default: `true`) — создать миниатюру 200x200
- `resize` (optional, default: `true`) — ресайз до 1024x768 с сохранением пропорций
- `watermark` (optional, default: `false`) — добавить водяной знак
- `watermark_text` (optional) — текст водяного знака (по умолчанию: `© ImageProcessor`)

**Пример:**
```bash
curl -X POST http://localhost:8034/api/images/upload \
  -F "file=@image.jpg" \
  -F "thumbnail=true" \
  -F "resize=true" \
  -F "watermark=true" \
  -F "watermark_text=My Watermark"
```

**Ответ:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "filename": "image.jpg",
  "status": "processing",
  "size": 245678,
  "created_at": "2026-02-05T15:30:45Z"
}
```

### `GET /api/images/{id}`
Получение обработанного изображения.

**Параметры:**
- `operation` (optional) — тип обработки: `thumbnail`, `resize`, `watermark` или пусто для оригинала

**Пример:**
```bash
# Оригинал
curl http://localhost:8034/api/images/550e8400-e29b-41d4-a716-446655440000

# Миниатюра
curl http://localhost:8034/api/images/550e8400-e29b-41d4-a716-446655440000?operation=thumbnail
```

### `GET /api/images/{id}/status`
Получение статуса обработки изображения.

**Ответ:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "completed"
}
```

### `DELETE /api/images/{id}`
Удаление изображения и всех его обработанных версий.

### `GET /api/images`
Список изображений с пагинацией.

**Параметры:**
- `limit` (default: 50, max: 100)
- `offset` (default: 0)

## Веб-интерфейс

Простой интерфейс для работы с сервисом:

- Загрузка изображений через форму
- Предпросмотр перед загрузкой
- Отображение статуса обработки (анимация для "в обработке")
- Просмотр и скачивание обработанных версий
- Удаление изображений
- Автоматическое обновление статуса через polling (каждые 5 сек)

Доступен по адресу: [http://localhost:8034](http://localhost:8034)


## Безопасность

- Валидация сигнатуры файла (магические числа) для защиты от подделки расширения
- Санитизация путей для предотвращения атак path traversal
- Санитизация имен файлов
- Ограничение размера загрузки (32 МБ)
- Валидация формата изображения на уровне сигнатуры и Content-Type

## Мониторинг

Логирование через `zerolog` с уровнями:
- `INFO` — успешные операции, запуск сервисов
- `WARN` — предупреждения (большой файл, невалидный формат)
- `ERROR` — ошибки обработки, инфраструктурные сбои
- `DEBUG` — детали обработки (отключено в продакшене)