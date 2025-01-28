#!/bin/bash

# Установка зависимостей Go
echo "Устанавливаем зависимости Go..."
go mod tidy

# Настройка базы данных
#!/bin/bash

# Переменные окружения передаются через workflow
DB_HOST=${POSTGRES_HOST:-localhost}
DB_PORT=${POSTGRES_PORT:-5432}
DB_USER=${POSTGRES_USER:-validator}
DB_PASSWORD=${POSTGRES_PASSWORD:-val1dat0r}
DB_NAME=${POSTGRES_DB:-project-sem-1}

echo "Ожидаем доступности PostgreSQL..."
until PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c '\q'; do
  >&2 echo "PostgreSQL ещё недоступен, ждём..."
  sleep 5
done
echo "PostgreSQL доступен!"

echo "Создаём таблицу 'prices', если её нет..."
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME <<EOF
CREATE TABLE IF NOT EXISTS prices (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    price NUMERIC NOT NULL,
    create_date DATE NOT NULL
);
EOF
echo "Таблица 'prices' готова!"
