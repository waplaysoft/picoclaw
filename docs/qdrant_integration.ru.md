# Интеграция PicoClaw с Qdrant

Этот документ описывает, как настроить PicoClaw для использования векторной базы данных Qdrant для хранения и поиска сообщений чата с эмбедингами от Mistral.

## Обзор

PicoClaw может хранить все сообщения чата в векторной базе данных Qdrant, что обеспечивает:
- **Постоянное хранение** всех разговоров между сессиями
- **Семантический поиск** по истории сообщений с использованием векторных эмбедингов
- **Извлечение контекста** для улучшения ответов ИИ

## Требования

1. **Экземпляр Qdrant** - работающая векторная база данных Qdrant (локальная или облачная)
2. **API-ключ Mistral** - для генерации эмбедингов с помощью модели `mistral-embed`

## Быстрый старт

### 1. Запуск Qdrant (локально)

```bash
docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant
```

### 2. Настройка PicoClaw

Отредактируйте `config.json` или используйте переменные окружения:

#### Через JSON конфиг:

```json
{
  "storage": {
    "qdrant": {
      "enabled": true,
      "host": "localhost",
      "port": 6333,
      "collection": "picoclaw_messages",
      "vector_size": 1024
    }
  },
  "model_list": [
    {
      "model_name": "mistral-embed",
      "model": "mistral/mistral-embed",
      "api_base": "https://api.mistral.ai/v1",
      "api_key": "ваш-api-ключ-mistral"
    }
  ]
}
```

#### Через переменные окружения:

```bash
# Включить хранилище Qdrant
export PICOCLAW_STORAGE_QDRANT_ENABLED=true
export PICOCLAW_STORAGE_QDRANT_HOST=localhost
export PICOCLAW_STORAGE_QDRANT_PORT=6333
export PICOCLAW_STORAGE_QDRANT_COLLECTION=picoclaw_messages
export PICOCLAW_STORAGE_QDRANT_VECTOR_SIZE=1024

# Mistral API для эмбедингов
export PICOCLAW_EMBEDDING_API_KEY=ваш-api-ключ-mistral
export PICOCLAW_EMBEDDING_MODEL=mistral-embed
```

### 3. Получение API-ключа Mistral

1. Посетите [Mistral AI Platform](https://console.mistral.ai/api-keys)
2. Создайте новый API-ключ
3. Добавьте его в конфигурацию

## Опции конфигурации

### Конфигурация Qdrant

| Поле | Переменная окружения | По умолчанию | Описание |
|------|---------------------|--------------|----------|
| `enabled` | `PICOCLAW_STORAGE_QDRANT_ENABLED` | `false` | Включить/выключить хранилище Qdrant |
| `host` | `PICOCLAW_STORAGE_QDRANT_HOST` | `localhost` | Имя сервера Qdrant |
| `port` | `PICOCLAW_STORAGE_QDRANT_PORT` | `6333` | HTTP порт Qdrant |
| `grpc_port` | `PICOCLAW_STORAGE_QDRANT_GRPC_PORT` | `6334` | gRPC порт Qdrant (опционально) |
| `api_key` | `PICOCLAW_STORAGE_QDRANT_API_KEY` | `""` | API-ключ для Qdrant Cloud |
| `collection` | `PICOCLAW_STORAGE_QDRANT_COLLECTION` | `picoclaw_messages` | Имя коллекции |
| `vector_size` | `PICOCLAW_STORAGE_QDRANT_VECTOR_SIZE` | `1024` | Размерность эмбединга (mistral-embed = 1024) |
| `secure` | `PICOCLAW_STORAGE_QDRANT_SECURE` | `false` | Использовать HTTPS |

### Конфигурация эмбедингов

| Поле | Переменная окружения | По умолчанию | Описание |
|------|---------------------|--------------|----------|
| `enabled` | `PICOCLAW_EMBEDDING_ENABLED` | `false` | Включить генерацию эмбедингов |
| `model` | `PICOCLAW_EMBEDDING_MODEL` | `mistral-embed` | Название модели эмбедингов |
| `api_base` | `PICOCLAW_EMBEDDING_API_BASE` | `https://api.mistral.ai/v1` | Конечная точка API |
| `api_key` | `PICOCLAW_EMBEDDING_API_KEY` | `""` | API-ключ для эмбедингов |

## Как это работает

1. **Хранение сообщений**: Когда сообщение получено, PicoClaw:
   - Отправляет содержимое сообщения в Mistral API для генерации эмбединга
   - Сохраняет сообщение с вектором эмбединга в Qdrant
   - Также сохраняет локально в JSON-файлы сессий

2. **Семантический поиск**: ИИ может искать похожие сообщения используя:
   - Векторное сходство (косинусное расстояние)
   - Фильтрацию по сессиям
   - Настраиваемые лимиты результатов

3. **Структура данных**: Каждое сохранённое сообщение содержит:
   - `session_key`: Уникальный идентификатор сессии
   - `role`: Роль сообщения (user/assistant/system)
   - `content`: Текст сообщения
   - `tool_calls`: Ассоциированные вызовы инструментов (если есть)
   - `timestamp`: Когда сообщение было сохранено
   - `message_index`: Позиция в разговоре

## Qdrant Cloud

Для использования Qdrant Cloud вместо локального экземпляра:

```json
{
  "storage": {
    "qdrant": {
      "enabled": true,
      "host": "ваш-cluster-id.cloud.qdrant.io",
      "port": 443,
      "api_key": "ваш-api-ключ-qdrant-cloud",
      "secure": true,
      "collection": "picoclaw_messages",
      "vector_size": 1024
    }
  }
}
```

## Решение проблем

### Ошибки подключения

- Убедитесь, что Qdrant запущен: `curl http://localhost:6333`
- Проверьте настройки файрвола для порта 6333
- Проверьте конфигурацию host/port

### Ошибки эмбедингов

- Убедитесь, что API-ключ Mistral действителен
- Проверьте лимиты API в [Mistral Dashboard](https://console.mistral.ai/)
- Убедитесь в наличии подключения к сети для `api.mistral.ai`

### Коллекция не создаётся

- Коллекция Qdrant создаётся автоматически при первом сообщении
- Проверьте логи PicoClaw на наличие ошибок создания
- Убедитесь, что в Qdrant достаточно места на диске

## Рекомендации по производительности

- **Генерация эмбедингов**: Каждое сообщение требует одного API-вызова к Mistral
- **Хранение векторов**: ~4KB на сообщение в Qdrant (1024 float32 + метаданные)
- **Скорость поиска**: Обычно <100мс для семантического поиска
- **Пакетные операции**: Несколько сообщений могут сохраняться пакетом для эффективности

## Заметки по безопасности

- Храните API-ключи безопасно, никогда не коммитьте в контроль версий
- Используйте Qdrant Cloud с HTTPS для production
- Учитывайте чувствительность содержимого сообщений перед сохранением
- Рекомендуется регулярное резервное копирование данных Qdrant

## Пример использования

После настройки все сообщения автоматически сохраняются. Для поиска:

```go
// В вашем коде используйте session manager
messages, err := sessionManager.SearchSimilarMessages(
    "session:123",                    // ключ сессии
    "Как установить Docker?",         // поисковый запрос
    5,                                // лимит результатов
)
```

## Лицензия

MIT License - Смотрите LICENSE основного проекта для деталей
