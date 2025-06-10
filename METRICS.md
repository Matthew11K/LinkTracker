# Метрики Link Tracker

## Бизнес-метрики

### 1. Пользовательские сообщения

#### Количество пользовательских сообщений
```promql
rate(link_tracker_bot_user_messages_rate[5m])
```

### 2. Активные ссылки в БД

#### График количества активных ссылок по типу
```promql
link_tracker_scrapper_active_links_count{link_type="github"}

link_tracker_scrapper_active_links_count{link_type="stackoverflow"}
```

### 3. Время работы scraping

#### p50, p95, p99 времени работы одного scrape по типу
```promql
histogram_quantile(0.50, rate(link_tracker_scrapper_scrape_request_duration_seconds_bucket[5m]))

histogram_quantile(0.95, rate(link_tracker_scrapper_scrape_request_duration_seconds_bucket[5m]))

histogram_quantile(0.99, rate(link_tracker_scrapper_scrape_request_duration_seconds_bucket[5m]))
```

## RED метрики

### Rate (Запросы в секунду)
```promql
rate(link_tracker_http_requests_total{service="$service"}[5m])
```

### Errors (Ошибки)
```promql
(
  sum(rate(link_tracker_http_requests_total{service="$service", status="error"}[1m])) or vector(0)
) / 
(
  sum(rate(link_tracker_http_requests_total{service="$service"}[1m])) or vector(1)
) * 100
```

### Duration (Длительность)
```promql
increase(link_tracker_http_request_duration_seconds_sum{service="$service"}[5m])
```

## Метрики памяти

### Использование памяти Go
```promql
go_memstats_heap_inuse_bytes{job=~"link-tracker-$service"}

go_memstats_stack_inuse_bytes{job=~"link-tracker-bot|link-tracker-scrapper"}

go_memstats_sys_bytes{job=~"link-tracker-bot|link-tracker-scrapper"}
```

## Переменные

### Service Variable
- Name: `service`
- Query: `label_values(link_tracker_http_requests_total, service)`