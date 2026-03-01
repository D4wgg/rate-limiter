## rate-limiter

Простой кластерный rate-limiter на Go, который проксирует HTTP‑запросы к другим сервисам и ограничивает их по RPS.

### Конфигурация

Основной конфиг — `config.yaml` (есть пример `config.example.yaml`):

```yaml
server:
  addr: ":8080"

routes:
  - route: "/api/foo"
    methods: ["GET"]
    upstream: "http://localhost:9000"
    limit:
      rps: 10
      window: 1s
```

- **server.addr**: адрес, на котором слушает rate‑limiter.
- **routes**:
  - **route**: путь, по которому внешний клиент ходит в кластер.
  - **methods**: список HTTP‑методов (если пусто — используются стандартные методы: GET/POST/PUT/DELETE/PATCH).
  - **upstream**: URL сервиса, на который будут проксироваться запросы.
  - **limit.rps**: максимально допустимое количество запросов в секунду.
  - **limit.window**: размер окна (обычно `1s` для RPS).

### Запуск локально

1. Установите Go 1.22+.
2. Создайте `config.yaml` по образцу `config.example.yaml`.
3. Соберите и запустите:

```bash
go run ./cmd/rate-limiter -config config.yaml
```

### Запуск в Docker / кластере

Собрать образ:

```bash
docker build -t rate-limiter:latest .
```

Запуск с внешним конфигом:

```bash
docker run --rm -p 8080:8080 ^
  -v %CD%/config.yaml:/app/config.yaml ^
  rate-limiter:latest
```

В Kubernetes/другом кластере достаточно:

- смонтировать `config.yaml` (ConfigMap/Volume),
- масштабировать Deployment — лимиты будут действовать **на инстанс** (примерно `RPS * количество реплик` для всего кластера).

