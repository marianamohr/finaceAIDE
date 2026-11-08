-- Seed: 12 meses de dados fictícios para o casal Ana + Bia
-- Contempla: balance_transfer, voucher_deposit correto, invoice_payment,
--             aportes em metas (reserva, viagem, carro), limites de categoria

DO $$
DECLARE
  v_user_ana   uuid;
  v_user_bia   uuid;
  v_account_id  uuid;

  -- Categorias
  cat_salario              uuid;
  cat_vr                   uuid;
  cat_freelance            uuid;
  cat_casa                 uuid;
  cat_comer_fora           uuid;
  cat_comida               uuid;
  cat_frutaria             uuid;
  cat_bebidas_nao_alc      uuid;
  cat_bebidas_alc          uuid;
  cat_higiene              uuid;
  cat_limpeza              uuid;
  cat_lazer                uuid;
  cat_vestuario            uuid;
  cat_beleza               uuid;
  cat_saude                uuid;
  cat_transporte           uuid;
  cat_pets                 uuid;
  cat_reserva              uuid;
  cat_outras_entradas      uuid;

  -- Formas de pagamento
  pm_nubank_ana   uuid;
  pm_nubank_bia   uuid;
  pm_conta         uuid;
  pm_vr_ana       uuid;
  pm_vr_bia       uuid;
  pm_dinheiro      uuid;

  -- Gastos fixos e assinaturas
  re_aluguel       uuid;
  re_condominio    uuid;
  re_plano_ana    uuid;
  re_plano_bia    uuid;
  re_netflix       uuid;
  re_spotify       uuid;
  re_disney        uuid;
  re_claude        uuid;
  re_amazon        uuid;

  -- Grupos de limites
  v_group_supermercado uuid;

  -- Metas
  v_goal_emergency  uuid;
  v_goal_viagem     uuid;
  v_goal_carro      uuid;

  -- Loop
  m             int;
  base_date     date;
  spend_mult    float;

  -- Balance transfer
  prev_net      float;

  -- Aportes metas
  reserve_amt   float;
  viagem_amt    float;
  carro_amt     float;

  -- Faturas
  nubank_ana_invoice  float;
  nubank_bia_invoice  float;
  prev_year            int;
  prev_month_int       int;

BEGIN

  -- ─── Resolver usuários ────────────────────────────────────────
  SELECT id, account_id INTO v_user_ana, v_account_id
  FROM users WHERE email = 'user@financeai.com';

  IF v_user_ana IS NULL THEN
    RAISE EXCEPTION 'Usuário user@financeai.com não encontrado';
  END IF;

  SELECT id INTO v_user_bia
  FROM users WHERE account_id = v_account_id AND email != 'user@financeai.com' LIMIT 1;

  IF v_user_bia IS NULL THEN
    v_user_bia := v_user_ana;
    RAISE NOTICE 'Segundo usuário não encontrado — usando Ana como Bia';
  END IF;

  -- ─── Resolver categorias ──────────────────────────────────────
  SELECT id INTO cat_salario         FROM categories WHERE slug = 'salario'                 LIMIT 1;
  SELECT id INTO cat_vr              FROM categories WHERE slug = 'vr'                      LIMIT 1;
  SELECT id INTO cat_freelance       FROM categories WHERE slug = 'freelance'               LIMIT 1;
  SELECT id INTO cat_casa            FROM categories WHERE slug = 'casa'                    LIMIT 1;
  SELECT id INTO cat_comer_fora      FROM categories WHERE slug = 'comer_fora'             LIMIT 1;
  SELECT id INTO cat_comida          FROM categories WHERE slug = 'comida'                  LIMIT 1;
  SELECT id INTO cat_frutaria        FROM categories WHERE slug = 'frutaria'                LIMIT 1;
  SELECT id INTO cat_bebidas_nao_alc FROM categories WHERE slug = 'bebidas_nao_alcoolicas' LIMIT 1;
  SELECT id INTO cat_bebidas_alc     FROM categories WHERE slug = 'bebidas_alcoolicas'      LIMIT 1;
  SELECT id INTO cat_higiene         FROM categories WHERE slug = 'higiene_pessoal'         LIMIT 1;
  SELECT id INTO cat_limpeza         FROM categories WHERE slug = 'limpeza_da_casa'         LIMIT 1;
  SELECT id INTO cat_lazer           FROM categories WHERE slug = 'lazer'                   LIMIT 1;
  SELECT id INTO cat_vestuario       FROM categories WHERE slug = 'vestuario'               LIMIT 1;
  SELECT id INTO cat_beleza          FROM categories WHERE slug = 'beleza'                  LIMIT 1;
  SELECT id INTO cat_saude           FROM categories WHERE slug = 'saude'                   LIMIT 1;
  SELECT id INTO cat_transporte      FROM categories WHERE slug = 'transporte'              LIMIT 1;
  SELECT id INTO cat_pets            FROM categories WHERE slug = 'pets'                    LIMIT 1;
  SELECT id INTO cat_reserva         FROM categories WHERE slug = 'reserva'                 LIMIT 1;
  SELECT id INTO cat_outras_entradas FROM categories WHERE slug = 'outras_entradas'         LIMIT 1;

  -- ─── Limpar todas as tabelas da conta (exceto users/accounts/passkeys) ──
  DELETE FROM transactions            WHERE account_id = v_account_id;
  DELETE FROM goal_transactions       WHERE account_id = v_account_id;
  DELETE FROM goals                   WHERE account_id = v_account_id;
  DELETE FROM account_category_limits WHERE account_id = v_account_id;
  DELETE FROM account_limit_groups    WHERE account_id = v_account_id;
  DELETE FROM recurring_expenses      WHERE account_id = v_account_id;
  DELETE FROM payment_methods         WHERE account_id = v_account_id;
  DELETE FROM chat_messages           WHERE account_id = v_account_id;
  DELETE FROM agent_insights          WHERE account_id = v_account_id;
  DELETE FROM pending_actions         WHERE account_id = v_account_id;
  DELETE FROM merchant_aliases        WHERE account_id = v_account_id;
  -- ─── Criar payment methods ────────────────────────────────────
  pm_nubank_ana := gen_random_uuid();
  INSERT INTO payment_methods (id, account_id, name, kind, closing_day, due_day, color, active, created_at)
  VALUES (pm_nubank_ana, v_account_id, 'Nubank Ana', 'credit_card', 26, 3, '#8B5CF6', true, now());

  pm_nubank_bia := gen_random_uuid();
  INSERT INTO payment_methods (id, account_id, name, kind, closing_day, due_day, color, active, created_at)
  VALUES (pm_nubank_bia, v_account_id, 'Nubank Bia', 'credit_card', 26, 3, '#6366F1', true, now());

  pm_conta := gen_random_uuid();
  INSERT INTO payment_methods (id, account_id, name, kind, color, active, created_at)
  VALUES (pm_conta, v_account_id, 'Conta corrente', 'debit', '#3B82F6', true, now());

  pm_vr_ana := gen_random_uuid();
  INSERT INTO payment_methods (id, account_id, name, kind, color, active, created_at)
  VALUES (pm_vr_ana, v_account_id, 'VR Ana', 'voucher', '#10B981', true, now());

  pm_vr_bia := gen_random_uuid();
  INSERT INTO payment_methods (id, account_id, name, kind, color, active, created_at)
  VALUES (pm_vr_bia, v_account_id, 'VR Bia', 'voucher', '#34D399', true, now());

  pm_dinheiro := gen_random_uuid();
  INSERT INTO payment_methods (id, account_id, name, kind, color, active, created_at)
  VALUES (pm_dinheiro, v_account_id, 'Dinheiro', 'cash', '#F59E0B', true, now());

  -- ─── Criar gastos fixos e assinaturas ────────────────────────
  re_aluguel    := gen_random_uuid();
  INSERT INTO recurring_expenses (id, account_id, name, amount, category_id, due_day, start_month, start_year, active, created_at, updated_at)
  VALUES (re_aluguel, v_account_id, 'Aluguel', 2800, cat_casa, 10, 6, 2025, true, now(), now());

  re_condominio := gen_random_uuid();
  INSERT INTO recurring_expenses (id, account_id, name, amount, category_id, due_day, start_month, start_year, active, created_at, updated_at)
  VALUES (re_condominio, v_account_id, 'Condomínio', 380, cat_casa, 10, 6, 2025, true, now(), now());

  re_plano_ana := gen_random_uuid();
  INSERT INTO recurring_expenses (id, account_id, name, amount, category_id, due_day, start_month, start_year, active, created_at, updated_at)
  VALUES (re_plano_ana, v_account_id, 'Plano de saúde Ana', 320, cat_saude, 1, 6, 2025, true, now(), now());

  re_plano_bia := gen_random_uuid();
  INSERT INTO recurring_expenses (id, account_id, name, amount, category_id, due_day, start_month, start_year, active, created_at, updated_at)
  VALUES (re_plano_bia, v_account_id, 'Plano de saúde Bia', 280, cat_saude, 1, 6, 2025, true, now(), now());

  re_netflix    := gen_random_uuid();
  INSERT INTO recurring_expenses (id, account_id, name, amount, category_id, payment_method_id, start_month, start_year, active, created_at, updated_at)
  VALUES (re_netflix, v_account_id, 'Netflix', 55.90, cat_lazer, pm_nubank_ana, 6, 2025, true, now(), now());

  re_spotify    := gen_random_uuid();
  INSERT INTO recurring_expenses (id, account_id, name, amount, category_id, payment_method_id, start_month, start_year, active, created_at, updated_at)
  VALUES (re_spotify, v_account_id, 'Spotify', 21.90, cat_lazer, pm_nubank_ana, 6, 2025, true, now(), now());

  re_disney     := gen_random_uuid();
  INSERT INTO recurring_expenses (id, account_id, name, amount, category_id, payment_method_id, start_month, start_year, active, created_at, updated_at)
  VALUES (re_disney, v_account_id, 'Disney', 65.00, cat_lazer, pm_nubank_ana, 6, 2025, true, now(), now());

  re_claude     := gen_random_uuid();
  INSERT INTO recurring_expenses (id, account_id, name, amount, category_id, payment_method_id, start_month, start_year, active, created_at, updated_at)
  VALUES (re_claude, v_account_id, 'Claude', 120.00, cat_lazer, pm_nubank_ana, 6, 2025, true, now(), now());

  re_amazon     := gen_random_uuid();
  INSERT INTO recurring_expenses (id, account_id, name, amount, category_id, payment_method_id, start_month, start_year, active, created_at, updated_at)
  VALUES (re_amazon, v_account_id, 'Amazon Prime', 30.00, cat_lazer, pm_nubank_ana, 6, 2025, true, now(), now());

  -- ─── Criar metas ──────────────────────────────────────────────
  v_goal_emergency := gen_random_uuid();
  INSERT INTO goals (id, account_id, name, target_amount, current_amount, initial_amount, monthly_contribution, annual_rate, goal_type, institution, account_type, created_at)
  VALUES (v_goal_emergency, v_account_id, 'Reserva de emergência', 60000, 0, 0, 700, 0.1150, 'emergency_global', 'Nubank', 'Caixinha', now());

  v_goal_viagem := gen_random_uuid();
  INSERT INTO goals (id, account_id, name, target_amount, current_amount, initial_amount, monthly_contribution, annual_rate, goal_type, institution, account_type, created_at)
  VALUES (v_goal_viagem, v_account_id, 'Viagem Europa', 25000, 0, 0, 400, 0.1150, 'travel', 'Nubank', 'Caixinha', now());

  v_goal_carro := gen_random_uuid();
  INSERT INTO goals (id, account_id, name, target_amount, current_amount, initial_amount, monthly_contribution, annual_rate, goal_type, institution, account_type, created_at)
  VALUES (v_goal_carro, v_account_id, 'Carro novo', 80000, 0, 0, 600, 0.1150, 'car', 'Nubank', 'Caixinha', now());

  -- ─── Limites de categoria (food_detail) ──────────────────────
  INSERT INTO account_category_limits (id, account_id, category_id, monthly_limit, created_at)
  VALUES (gen_random_uuid(), v_account_id, cat_comer_fora,      1200, now())
  ON CONFLICT (account_id, category_id) DO UPDATE SET monthly_limit = EXCLUDED.monthly_limit;

  -- Grupo "Supermercado" agrupa comida, frutaria, bebidas não alc, higiene e limpeza
  v_group_supermercado := gen_random_uuid();
  INSERT INTO account_limit_groups (id, account_id, label, created_at, updated_at)
  VALUES (v_group_supermercado, v_account_id, 'Supermercado', now(), now());

  INSERT INTO account_category_limits (id, account_id, category_id, group_id, monthly_limit, created_at)
  VALUES (gen_random_uuid(), v_account_id, cat_comida,         v_group_supermercado, 1000, now());

  INSERT INTO account_category_limits (id, account_id, category_id, group_id, monthly_limit, created_at)
  VALUES (gen_random_uuid(), v_account_id, cat_frutaria,       v_group_supermercado,  200, now());

  INSERT INTO account_category_limits (id, account_id, category_id, group_id, monthly_limit, created_at)
  VALUES (gen_random_uuid(), v_account_id, cat_bebidas_nao_alc, v_group_supermercado, 150, now());

  INSERT INTO account_category_limits (id, account_id, category_id, group_id, monthly_limit, created_at)
  VALUES (gen_random_uuid(), v_account_id, cat_higiene,        v_group_supermercado,  200, now());

  INSERT INTO account_category_limits (id, account_id, category_id, group_id, monthly_limit, created_at)
  VALUES (gen_random_uuid(), v_account_id, cat_limpeza,        v_group_supermercado,  150, now());

  INSERT INTO account_category_limits (id, account_id, category_id, monthly_limit, created_at)
  VALUES (gen_random_uuid(), v_account_id, cat_bebidas_alc,    400, now())
  ON CONFLICT (account_id, category_id) DO UPDATE SET monthly_limit = EXCLUDED.monthly_limit;

  INSERT INTO account_category_limits (id, account_id, category_id, monthly_limit, created_at)
  VALUES (gen_random_uuid(), v_account_id, cat_transporte,       600, now())
  ON CONFLICT (account_id, category_id) DO UPDATE SET monthly_limit = EXCLUDED.monthly_limit;

  INSERT INTO account_category_limits (id, account_id, category_id, monthly_limit, created_at)
  VALUES (gen_random_uuid(), v_account_id, cat_lazer,            800, now())
  ON CONFLICT (account_id, category_id) DO UPDATE SET monthly_limit = EXCLUDED.monthly_limit;

  INSERT INTO account_category_limits (id, account_id, category_id, monthly_limit, created_at)
  VALUES (gen_random_uuid(), v_account_id, cat_beleza,           400, now())
  ON CONFLICT (account_id, category_id) DO UPDATE SET monthly_limit = EXCLUDED.monthly_limit;

  INSERT INTO account_category_limits (id, account_id, category_id, monthly_limit, created_at)
  VALUES (gen_random_uuid(), v_account_id, cat_saude,            700, now())
  ON CONFLICT (account_id, category_id) DO UPDATE SET monthly_limit = EXCLUDED.monthly_limit;

  INSERT INTO account_category_limits (id, account_id, category_id, monthly_limit, created_at)
  VALUES (gen_random_uuid(), v_account_id, cat_vestuario,        600, now())
  ON CONFLICT (account_id, category_id) DO UPDATE SET monthly_limit = EXCLUDED.monthly_limit;

  INSERT INTO account_category_limits (id, account_id, category_id, monthly_limit, created_at)
  VALUES (gen_random_uuid(), v_account_id, cat_casa,            4500, now())
  ON CONFLICT (account_id, category_id) DO UPDATE SET monthly_limit = EXCLUDED.monthly_limit;

  INSERT INTO account_category_limits (id, account_id, category_id, monthly_limit, created_at)
  VALUES (gen_random_uuid(), v_account_id, cat_pets,             300, now())
  ON CONFLICT (account_id, category_id) DO UPDATE SET monthly_limit = EXCLUDED.monthly_limit;

-- ─── Loop mensal ──────────────────────────────────────────────
FOR m IN 0..11 LOOP
  base_date := date_trunc('month', current_date) - ((11 - m) || ' months')::interval;

  -- Multiplicador sazonal
  IF EXTRACT(MONTH FROM base_date) IN (10, 11, 12) THEN
    spend_mult := 1.25;
  ELSIF EXTRACT(MONTH FROM base_date) IN (1, 2) THEN
    spend_mult := 0.85;
  ELSE
    spend_mult := 1.0 + (random() * 0.2 - 0.1);
  END IF;

  -- ── 1. Balance transfer do mês anterior ───────────────────────
  IF m > 0 THEN
    SELECT COALESCE(SUM(amount), 0) INTO prev_net
    FROM transactions
    WHERE account_id = v_account_id
      AND date >= base_date - INTERVAL '1 month'
      AND date  < base_date
      AND type  != 'transfer';

    IF prev_net > 0 THEN
      INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, locale, currency, original_amount, created_at)
      VALUES (
        gen_random_uuid(), v_user_ana, v_account_id,
        prev_net, 'outras_entradas', cat_outras_entradas,
        'Conta Corrente', 'Saldo do mês anterior',
        base_date,
        'balance_transfer', 'regular', 'BR', 'BRL', prev_net, base_date
      );
    END IF;
  END IF;

  -- ── 2. Pagamento de fatura do mês anterior (Nubank Ana + Bia) ──
  IF m > 0 THEN
    prev_year      := EXTRACT(YEAR  FROM (base_date - INTERVAL '1 month'))::int;
    prev_month_int := EXTRACT(MONTH FROM (base_date - INTERVAL '1 month'))::int;

    -- Fatura Nubank Ana
    SELECT COALESCE(SUM(t.amount), 0) INTO nubank_ana_invoice
    FROM transactions t
    WHERE t.account_id = v_account_id
      AND t.payment_method_id = pm_nubank_ana
      AND t.type != 'invoice_payment'
      AND (
        (EXTRACT(DAY FROM t.date) <= 26
         AND EXTRACT(YEAR  FROM t.date)::int  = prev_year
         AND EXTRACT(MONTH FROM t.date)::int  = prev_month_int)
        OR
        (EXTRACT(DAY FROM t.date) > 26
         AND EXTRACT(YEAR  FROM (t.date + INTERVAL '1 month'))::int  = prev_year
         AND EXTRACT(MONTH FROM (t.date + INTERVAL '1 month'))::int  = prev_month_int)
      );

    IF nubank_ana_invoice < 0 THEN
      INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, billing_year, billing_month, locale, currency, original_amount, created_at)
      VALUES (
        gen_random_uuid(), v_user_ana, v_account_id,
        nubank_ana_invoice, '', NULL,
        'Nubank', 'Fatura Nubank Ana',
        base_date + 2,
        'manual', 'invoice_payment',
        pm_nubank_ana, prev_year, prev_month_int,
        'BR', 'BRL', ABS(nubank_ana_invoice), base_date + 2
      );
    END IF;

    -- Fatura Nubank Bia
    SELECT COALESCE(SUM(t.amount), 0) INTO nubank_bia_invoice
    FROM transactions t
    WHERE t.account_id = v_account_id
      AND t.payment_method_id = pm_nubank_bia
      AND t.type != 'invoice_payment'
      AND (
        (EXTRACT(DAY FROM t.date) <= 26
         AND EXTRACT(YEAR  FROM t.date)::int  = prev_year
         AND EXTRACT(MONTH FROM t.date)::int  = prev_month_int)
        OR
        (EXTRACT(DAY FROM t.date) > 26
         AND EXTRACT(YEAR  FROM (t.date + INTERVAL '1 month'))::int  = prev_year
         AND EXTRACT(MONTH FROM (t.date + INTERVAL '1 month'))::int  = prev_month_int)
      );

    IF nubank_bia_invoice < 0 THEN
      INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, billing_year, billing_month, locale, currency, original_amount, created_at)
      VALUES (
        gen_random_uuid(), v_user_bia, v_account_id,
        nubank_bia_invoice, '', NULL,
        'Nubank', 'Fatura Nubank Bia',
        base_date + 2,
        'manual', 'invoice_payment',
        pm_nubank_bia, prev_year, prev_month_int,
        'BR', 'BRL', ABS(nubank_bia_invoice), base_date + 2
      );
    END IF;
  END IF;

  -- ── 3. Receitas ───────────────────────────────────────────────

  -- Salário Ana (dia 5)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, locale, currency, original_amount, created_at)
  VALUES (
    gen_random_uuid(), v_user_ana, v_account_id,
    9200 + (random() * 800),
    'salario', cat_salario,
    'Empresa ABC', 'Salário Ana',
    base_date + 4,
    'manual', 'regular', 'BR', 'BRL', 9200 + (random() * 800), base_date + 4
  );

  -- Salário Bia (dia 5)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, locale, currency, original_amount, created_at)
  VALUES (
    gen_random_uuid(), v_user_bia, v_account_id,
    8000 + (random() * 800),
    'salario', cat_salario,
    'Empresa XYZ', 'Salário Bia',
    base_date + 4,
    'manual', 'regular', 'BR', 'BRL', 8000 + (random() * 800), base_date + 4
  );

  -- VR Ana (dia 5) — voucher_deposit: voucher_payment_method_id, payment_method_id=NULL
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, voucher_payment_method_id, locale, currency, original_amount, created_at)
  VALUES (
    gen_random_uuid(), v_user_ana, v_account_id,
    650,
    'vr', cat_vr,
    'Empresa ABC', 'Vale Refeição Ana',
    base_date + 4,
    'voucher_deposit', 'regular',
    pm_vr_ana,
    'BR', 'BRL', 650, base_date + 4
  );

  -- VR Bia (dia 5)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, voucher_payment_method_id, locale, currency, original_amount, created_at)
  VALUES (
    gen_random_uuid(), v_user_bia, v_account_id,
    600,
    'vr', cat_vr,
    'Empresa XYZ', 'Vale Refeição Bia',
    base_date + 4,
    'voucher_deposit', 'regular',
    pm_vr_bia,
    'BR', 'BRL', 600, base_date + 4
  );

  -- Freelance Ana (50% dos meses)
  IF random() > 0.5 THEN
    INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, locale, currency, original_amount, created_at)
    VALUES (
      gen_random_uuid(), v_user_ana, v_account_id,
      800 + random() * 1200,
      'freelance', cat_freelance,
      'Cliente Freelance', 'Projeto freelance Ana',
      base_date + 14 + (random() * 5)::int,
      'manual', 'regular', 'BR', 'BRL', 800 + random() * 1200, base_date + 14
    );
  END IF;

  -- Freelance Bia (30% dos meses)
  IF random() > 0.7 THEN
    INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, locale, currency, original_amount, created_at)
    VALUES (
      gen_random_uuid(), v_user_bia, v_account_id,
      500 + random() * 1000,
      'freelance', cat_freelance,
      'Cliente Freelance', 'Projeto freelance Bia',
      base_date + 19 + (random() * 5)::int,
      'manual', 'regular', 'BR', 'BRL', 500 + random() * 1000, base_date + 19
    );
  END IF;

  -- ── 4. Despesas fixas ─────────────────────────────────────────

  -- Plano de saúde Ana (dia 1)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, date, source, type, payment_method_id, recurring_expense_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -320, 'saude', cat_saude, 'Plano de saúde Ana', base_date, 'recurring', 'regular', pm_conta, re_plano_ana, 'BR', 'BRL', 320, base_date);

  -- Plano de saúde Bia (dia 1)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, date, source, type, payment_method_id, recurring_expense_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_bia, v_account_id, -280, 'saude', cat_saude, 'Plano de saúde Bia', base_date, 'recurring', 'regular', pm_conta, re_plano_bia, 'BR', 'BRL', 280, base_date);

  -- Aluguel (dia 10)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, date, source, type, payment_method_id, recurring_expense_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -2800, 'casa', cat_casa, 'Aluguel', base_date + 9, 'recurring', 'regular', pm_conta, re_aluguel, 'BR', 'BRL', 2800, base_date + 9);

  -- Condomínio (dia 10)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, date, source, type, payment_method_id, recurring_expense_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -380, 'casa', cat_casa, 'Condomínio', base_date + 9, 'recurring', 'regular', pm_conta, re_condominio, 'BR', 'BRL', 380, base_date + 9);

  -- Luz (dia 10)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -(80 + random() * 60), 'casa', cat_casa, 'ENEL', 'Conta de luz', base_date + 9, 'manual', 'regular', pm_conta, 'BR', 'BRL', 80 + random() * 60, base_date + 9);

  -- Internet (dia 15, Nubank Ana)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -(120 + random() * 30), 'casa', cat_casa, 'Vivo', 'Internet fibra', base_date + 14, 'manual', 'regular', pm_nubank_ana, 'BR', 'BRL', 120 + random() * 30, base_date + 14);

  -- ── 5. Alimentação ────────────────────────────────────────────

  -- Supermercado 1 — comida (dia 6, Nubank Ana)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -(180 + random() * 80) * spend_mult, 'comida', cat_comida, 'Pão de Açúcar', 'Compras do mês', base_date + 5, 'manual', 'regular', pm_nubank_ana, 'BR', 'BRL', (180 + random() * 80) * spend_mult, base_date + 5);

  -- Supermercado 2 — comida (dia 20, débito)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_bia, v_account_id, -(120 + random() * 60), 'comida', cat_comida, 'Atacadão', 'Compras quinzenais', base_date + 19, 'manual', 'regular', pm_conta, 'BR', 'BRL', 120 + random() * 60, base_date + 19);

  -- Frutaria (dia 13, dinheiro)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -(40 + random() * 30), 'frutaria', cat_frutaria, 'Frutaria do bairro', 'Frutas e verduras', base_date + 12, 'manual', 'regular', pm_dinheiro, 'BR', 'BRL', 40 + random() * 30, base_date + 12);

  -- Higiene pessoal (dia 8, Nubank Ana)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -(60 + random() * 40), 'higiene_pessoal', cat_higiene, 'Drogaria', 'Higiene pessoal', base_date + 7, 'manual', 'regular', pm_nubank_ana, 'BR', 'BRL', 60 + random() * 40, base_date + 7);

  -- Limpeza da casa (dia 8, débito)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_bia, v_account_id, -(50 + random() * 30), 'limpeza_da_casa', cat_limpeza, 'Supermercado', 'Produtos de limpeza', base_date + 7, 'manual', 'regular', pm_conta, 'BR', 'BRL', 50 + random() * 30, base_date + 7);

  -- Bebidas não alcoólicas (dia 10, débito)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -(30 + random() * 30), 'bebidas_nao_alcoolicas', cat_bebidas_nao_alc, 'Supermercado', 'Água, sucos, café', base_date + 9, 'manual', 'regular', pm_conta, 'BR', 'BRL', 30 + random() * 30, base_date + 9);

  -- iFood delivery 1 (dia 1, VR Ana — comer_fora)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -(100 + random() * 30), 'comer_fora', cat_comer_fora, 'iFood', 'Almoço delivery', base_date + 0, 'manual', 'regular', pm_vr_ana, 'BR', 'BRL', 100 + random() * 30, base_date + 0);

  -- iFood delivery 2 (dia 11, VR Ana)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -(90 + random() * 30), 'comer_fora', cat_comer_fora, 'iFood', 'Almoço delivery', base_date + 10, 'manual', 'regular', pm_vr_ana, 'BR', 'BRL', 90 + random() * 30, base_date + 10);

  -- iFood delivery 3 (dia 17, VR Bia)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_bia, v_account_id, -(85 + random() * 30), 'comer_fora', cat_comer_fora, 'iFood', 'Almoço delivery', base_date + 16, 'manual', 'regular', pm_vr_bia, 'BR', 'BRL', 85 + random() * 30, base_date + 16);

  -- Restaurante jantar (dia 3, Nubank Ana)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -(40 + random() * 40), 'comer_fora', cat_comer_fora, 'Restaurante', 'Jantar fora', base_date + 2, 'manual', 'regular', pm_nubank_ana, 'BR', 'BRL', 40 + random() * 40, base_date + 2);

  -- Restaurante almoço Bia (dia 15, Nubank Bia)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_bia, v_account_id, -(60 + random() * 40), 'comer_fora', cat_comer_fora, 'Outback', 'Almoço de semana', base_date + 14, 'manual', 'regular', pm_nubank_bia, 'BR', 'BRL', 60 + random() * 40, base_date + 14);

  -- Jantar fora casal (dia 25, Nubank Ana)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -(40 + random() * 20), 'comer_fora', cat_comer_fora, 'Cantinho do Frango', 'Jantar casal', base_date + 24, 'manual', 'regular', pm_nubank_ana, 'BR', 'BRL', 40 + random() * 20, base_date + 24);

  -- ── 6. Transporte ─────────────────────────────────────────────
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -(18 + random() * 22), 'transporte', cat_transporte, 'Uber', 'Corrida Uber', base_date + 1, 'manual', 'regular', pm_nubank_ana, 'BR', 'BRL', 18 + random() * 22, base_date + 1);

  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_bia, v_account_id, -(15 + random() * 20), 'transporte', cat_transporte, 'Uber', 'Corrida Uber', base_date + 8, 'manual', 'regular', pm_nubank_bia, 'BR', 'BRL', 15 + random() * 20, base_date + 8);

  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -(20 + random() * 25), 'transporte', cat_transporte, 'Uber', 'Corrida Uber', base_date + 15, 'manual', 'regular', pm_nubank_ana, 'BR', 'BRL', 20 + random() * 25, base_date + 15);

  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_bia, v_account_id, -(12 + random() * 18), 'transporte', cat_transporte, '99', 'Corrida 99', base_date + 22, 'manual', 'regular', pm_nubank_bia, 'BR', 'BRL', 12 + random() * 18, base_date + 22);

  -- ── 7. Lazer ──────────────────────────────────────────────────

  -- Netflix (dia 1, assinatura cartão)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, date, source, type, payment_method_id, recurring_expense_id, billing_year, billing_month, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -55.90, 'lazer', cat_lazer, 'Netflix', base_date, 'recurring', 'regular', pm_nubank_ana, re_netflix,
    EXTRACT(YEAR FROM base_date)::int, EXTRACT(MONTH FROM base_date)::int, 'BR', 'BRL', 55.90, base_date);

  -- Spotify (dia 1, assinatura cartão)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, date, source, type, payment_method_id, recurring_expense_id, billing_year, billing_month, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -21.90, 'lazer', cat_lazer, 'Spotify', base_date, 'recurring', 'regular', pm_nubank_ana, re_spotify,
    EXTRACT(YEAR FROM base_date)::int, EXTRACT(MONTH FROM base_date)::int, 'BR', 'BRL', 21.90, base_date);

  -- Disney (dia 1, assinatura cartão)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, date, source, type, payment_method_id, recurring_expense_id, billing_year, billing_month, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -65.00, 'lazer', cat_lazer, 'Disney', base_date, 'recurring', 'regular', pm_nubank_ana, re_disney,
    EXTRACT(YEAR FROM base_date)::int, EXTRACT(MONTH FROM base_date)::int, 'BR', 'BRL', 65.00, base_date);

  -- Claude (dia 1, assinatura cartão)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, date, source, type, payment_method_id, recurring_expense_id, billing_year, billing_month, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -120.00, 'lazer', cat_lazer, 'Claude', base_date, 'recurring', 'regular', pm_nubank_ana, re_claude,
    EXTRACT(YEAR FROM base_date)::int, EXTRACT(MONTH FROM base_date)::int, 'BR', 'BRL', 120.00, base_date);

  -- Amazon Prime (dia 1, assinatura cartão)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, date, source, type, payment_method_id, recurring_expense_id, billing_year, billing_month, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -30.00, 'lazer', cat_lazer, 'Amazon Prime', base_date, 'recurring', 'regular', pm_nubank_ana, re_amazon,
    EXTRACT(YEAR FROM base_date)::int, EXTRACT(MONTH FROM base_date)::int, 'BR', 'BRL', 30.00, base_date);

  -- Cinema (dia 12)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -(60 + random() * 80) * spend_mult, 'lazer', cat_lazer, 'Cinemark', 'Cinema e pipoca', base_date + 11, 'manual', 'regular', pm_nubank_ana, 'BR', 'BRL', (60 + random() * 80) * spend_mult, base_date + 11);

  -- Bar saída (dia 5)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -(80 + random() * 120) * spend_mult, 'lazer', cat_lazer, 'Bar do Zé', 'Saída com amigos', base_date + 4, 'manual', 'regular', pm_nubank_ana, 'BR', 'BRL', (80 + random() * 120) * spend_mult, base_date + 4);

  -- ── 8. Bebidas alcoólicas ─────────────────────────────────────
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -(50 + random() * 100) * spend_mult, 'bebidas_alcoolicas', cat_bebidas_alc, 'Bar & Drinks', 'Cervejas fim de semana', base_date + 6, 'manual', 'regular', pm_nubank_ana, 'BR', 'BRL', (50 + random() * 100) * spend_mult, base_date + 6);

  -- Cerveja Bia (dia 21)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_bia, v_account_id, -(40 + random() * 60) * spend_mult, 'bebidas_alcoolicas', cat_bebidas_alc, 'Boteco', 'Cervejas', base_date + 20, 'manual', 'regular', pm_nubank_bia, 'BR', 'BRL', (40 + random() * 60) * spend_mult, base_date + 20);

  -- ── 9. Vestuário ──────────────────────────────────────────────
  IF m % 2 = 0 THEN
    INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
    VALUES (gen_random_uuid(), v_user_ana, v_account_id, -(150 + random() * 300) * spend_mult, 'vestuario', cat_vestuario, 'Zara', 'Roupas e acessórios', base_date + 12, 'manual', 'regular', pm_nubank_ana, 'BR', 'BRL', (150 + random() * 300) * spend_mult, base_date + 12);
  END IF;

  IF m % 3 = 1 THEN
    INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
    VALUES (gen_random_uuid(), v_user_bia, v_account_id, -(100 + random() * 150), 'vestuario', cat_vestuario, 'Renner', 'Compras de roupa', base_date + 18, 'manual', 'regular', pm_nubank_bia, 'BR', 'BRL', 100 + random() * 150, base_date + 18);
  END IF;

  -- ── 10. Beleza ────────────────────────────────────────────────

  -- Salão (dia 18, Nubank Ana)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -(120 + random() * 80) * (CASE WHEN m % 2 = 0 THEN 1.3 ELSE 1.0 END), 'beleza', cat_beleza, 'Studio de Beleza', 'Salão / estética', base_date + 17, 'manual', 'regular', pm_nubank_ana, 'BR', 'BRL', (120 + random() * 80) * (CASE WHEN m % 2 = 0 THEN 1.3 ELSE 1.0 END), base_date + 17);

  -- Manicure 1 (dia 4)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -(55 + random() * 20), 'beleza', cat_beleza, 'Manicure', 'Manicure/pedicure', base_date + 3, 'manual', 'regular', pm_conta, 'BR', 'BRL', 55 + random() * 20, base_date + 3);

  -- Manicure 2 (dia 19)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -(55 + random() * 20), 'beleza', cat_beleza, 'Manicure', 'Manicure/pedicure', base_date + 18, 'manual', 'regular', pm_conta, 'BR', 'BRL', 55 + random() * 20, base_date + 18);

  -- ── 11. Saúde ─────────────────────────────────────────────────

  -- Farmácia (dia 8, Nubank Ana)
  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -(60 + random() * 80), 'saude', cat_saude, 'Drogasil', 'Remédios', base_date + 7, 'manual', 'regular', pm_nubank_ana, 'BR', 'BRL', 60 + random() * 80, base_date + 7);

  -- Consulta médica / dentista (a cada 2 meses)
  IF m % 2 = 0 THEN
    INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
    VALUES (
      gen_random_uuid(), v_user_ana, v_account_id,
      -(150 + random() * 100), 'saude', cat_saude,
      'Consultório', CASE WHEN m % 4 = 0 THEN 'Consulta médica' ELSE 'Dentista' END,
      base_date + 15,
      'manual', 'regular', pm_nubank_ana, 'BR', 'BRL', 150 + random() * 100, base_date + 15
    );
  END IF;

  -- ── 12. Pets — Bowie (a cada 2 meses) ────────────────────────
  IF m % 2 = 0 THEN
    INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, payment_method_id, locale, currency, original_amount, created_at)
    VALUES (
      gen_random_uuid(), v_user_ana, v_account_id,
      -(80 + random() * 100), 'pets', cat_pets,
      CASE WHEN m % 4 = 0 THEN 'Petshop' ELSE 'Veterinária' END,
      CASE WHEN m % 4 = 0 THEN 'Ração Bowie' ELSE 'Consulta Bowie' END,
      base_date + 20,
      'manual', 'regular', pm_nubank_ana, 'BR', 'BRL', 80 + random() * 100, base_date + 20
    );
  END IF;

  -- ── 13. Aportes nas metas (fim do mês) ────────────────────────

  -- Reserva de emergência
  reserve_amt := 500 + random() * 300;

  INSERT INTO goal_transactions (id, goal_id, account_id, amount, description, date, source, created_at)
  VALUES (gen_random_uuid(), v_goal_emergency, v_account_id, reserve_amt, 'Aporte mensal reserva emergência', base_date + 27, 'manual', base_date + 27);

  UPDATE goals SET current_amount = current_amount + reserve_amt WHERE id = v_goal_emergency;

  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -reserve_amt, 'reserva', cat_reserva, 'Reserva', 'Aporte reserva emergência', base_date + 27, 'reserve', 'transfer', 'BR', 'BRL', reserve_amt, base_date + 27);

  -- Viagem Europa
  viagem_amt := 350 + random() * 100;

  INSERT INTO goal_transactions (id, goal_id, account_id, amount, description, date, source, created_at)
  VALUES (gen_random_uuid(), v_goal_viagem, v_account_id, viagem_amt, 'Aporte mensal viagem Europa', base_date + 27, 'manual', base_date + 27);

  UPDATE goals SET current_amount = current_amount + viagem_amt WHERE id = v_goal_viagem;

  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_ana, v_account_id, -viagem_amt, 'reserva', cat_reserva, 'Reserva', 'Aporte viagem Europa', base_date + 27, 'reserve', 'transfer', 'BR', 'BRL', viagem_amt, base_date + 27);

  -- Carro novo
  carro_amt := 550 + random() * 100;

  INSERT INTO goal_transactions (id, goal_id, account_id, amount, description, date, source, created_at)
  VALUES (gen_random_uuid(), v_goal_carro, v_account_id, carro_amt, 'Aporte mensal carro novo', base_date + 27, 'manual', base_date + 27);

  UPDATE goals SET current_amount = current_amount + carro_amt WHERE id = v_goal_carro;

  INSERT INTO transactions (id, user_id, account_id, amount, category, category_id, merchant, description, date, source, type, locale, currency, original_amount, created_at)
  VALUES (gen_random_uuid(), v_user_bia, v_account_id, -carro_amt, 'reserva', cat_reserva, 'Reserva', 'Aporte carro novo', base_date + 27, 'reserve', 'transfer', 'BR', 'BRL', carro_amt, base_date + 27);

END LOOP;

-- ─── Pós-loop: billing_year/billing_month para cartões ────────
UPDATE transactions t
SET
  billing_year  = CASE
    WHEN EXTRACT(DAY FROM t.date) <= 26
    THEN EXTRACT(YEAR  FROM t.date)::int
    ELSE EXTRACT(YEAR  FROM (t.date + INTERVAL '1 month'))::int
  END,
  billing_month = CASE
    WHEN EXTRACT(DAY FROM t.date) <= 26
    THEN EXTRACT(MONTH FROM t.date)::int
    ELSE EXTRACT(MONTH FROM (t.date + INTERVAL '1 month'))::int
  END
FROM payment_methods pm
WHERE t.payment_method_id = pm.id
  AND pm.kind = 'credit_card'
  AND t.type  != 'invoice_payment'
  AND t.billing_year IS NULL
  AND t.account_id = v_account_id;

RAISE NOTICE 'Seed concluído com sucesso!';
END $$;

-- ─── Backfill category_id (sem UUID hardcoded) ────────────────
UPDATE transactions t
SET category_id = c.id
FROM categories c
WHERE t.category_id IS NULL
  AND c.slug = t.category
  AND t.account_id IN (
    SELECT account_id FROM users WHERE email = 'user@financeai.com'
  );
