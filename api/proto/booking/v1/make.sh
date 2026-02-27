#!/usr/bin/env bash
set -euo pipefail

# Скрипт генерирует Go + gRPC файлы рядом с .proto файлом (source_relative).
# По умолчанию файл booking.proto в текущей папке. Можно передать имя файла как аргумент.
#
# При необходимости задайте переменную окружения PROTO_INCLUDE (пути через ':'), чтобы
# добавить дополнительные --proto_path (например /usr/local/include или директорию с googleapis),
# если protoc не находит google/protobuf/timestamp.proto.

PROTO_FILE="${1:-booking.proto}"

if [ ! -f "$PROTO_FILE" ]; then
  echo "Файл '$PROTO_FILE' не найден в текущей папке."
  exit 2
fi

# Проверки
if ! command -v protoc >/dev/null 2>&1; then
  echo "Ошибка: protoc не установлен или не в PATH."
  echo "Установите protoc: https://grpc.io/docs/protoc-installation/"
  exit 3
fi

# Проверяем и устанавливаем плагины при отсутствии
if ! command -v protoc-gen-go >/dev/null 2>&1; then
  echo "protoc-gen-go не найден, устанавливаю (go install google.golang.org/protobuf/cmd/protoc-gen-go@latest)..."
  if ! command -v go >/dev/null 2>&1; then
    echo "go не установлен или не в PATH. Установите Go для автоматической установки плагинов."
    exit 4
  fi
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

if ! command -v protoc-gen-go-grpc >/dev/null 2>&1; then
  echo "protoc-gen-go-grpc не найден, устанавливаю (go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest)..."
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

# Подготовка proto_path аргументов
DIR="$(cd "$(dirname "$PROTO_FILE")" && pwd)"
PROTO_PATH_ARGS=("-I" "$DIR")

# Если задана переменная окружения PROTO_INCLUDE, добавляем ее (пути через ':')
if [ -n "${PROTO_INCLUDE:-}" ]; then
  IFS=':' read -r -a EXTRA_PATHS <<< "$PROTO_INCLUDE"
  for p in "${EXTRA_PATHS[@]}"; do
    PROTO_PATH_ARGS+=("-I" "$p")
  done
fi

# Еще можно добавить /usr/local/include как часто используемый путь установки protobuf
if [ -d "/usr/local/include" ]; then
  PROTO_PATH_ARGS+=("-I" "/usr/local/include")
fi

echo "Запускаю protoc для файла: $PROTO_FILE"
echo "proto_path: ${PROTO_PATH_ARGS[*]}"

protoc "${PROTO_PATH_ARGS[@]}" \
  --go_out="$DIR" --go_opt=paths=source_relative \
  --go-grpc_out="$DIR" --go-grpc_opt=paths=source_relative \
  "$PROTO_FILE"

echo "Генерация завершена. Сгенерированные файлы расположены рядом с $PROTO_FILE"