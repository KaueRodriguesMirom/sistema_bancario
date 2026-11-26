# Banco Demo

Aplicação bancária demonstrativa com Flutter/Dart, Go e PostgreSQL.

## Recursos
- Cadastro e login
- Saldo em tempo real
- Depósito
- Saque rápido
- Histórico das últimas 50 operações
- Operações em transação de banco de dados

## Executar

```bash
docker compose up --build
```

API: `http://localhost:8080/api/health`

A conta usa valores em centavos para evitar problemas de arredondamento.
