-- Rollback manual do módulo Dispensa (dispensaAI).
-- O projeto não tem migrations reversíveis (dívida técnica aceita em 2026-09-07).
-- Execute este arquivo para remover TODAS as tabelas pantry_* e seus dados.
-- Ordem: filhas antes das pais (FKs com ON DELETE CASCADE ainda exigem a ordem para DROP sem CASCADE).

BEGIN;

-- Etapa 4 — receitas
DROP TABLE IF EXISTS pantry_recipe_logs;
DROP TABLE IF EXISTS pantry_recipes;

-- Etapa 3 — lista de compras automática
DROP TABLE IF EXISTS pantry_shopping_items;

-- Etapa 2 — aliases e leitura de nota fiscal
DROP TABLE IF EXISTS pantry_receipt_items;
DROP TABLE IF EXISTS pantry_receipts;
DROP TABLE IF EXISTS pantry_product_aliases;

-- Etapa 1 — catálogo, estoque e movimentos
DROP TABLE IF EXISTS pantry_movements;
DROP TABLE IF EXISTS pantry_stock;
DROP TABLE IF EXISTS pantry_products;

-- Etapa 0 — fundação
DROP TABLE IF EXISTS pantry_seed_products;
DROP TABLE IF EXISTS pantry_settings;

COMMIT;
