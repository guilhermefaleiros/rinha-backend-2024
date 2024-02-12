SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

CREATE UNLOGGED TABLE clientes (
                          id SERIAL PRIMARY KEY,
                          nome VARCHAR(50) NOT NULL,
                          limite INTEGER NOT NULL,
                          saldo INTEGER NOT NULL
) WITH (autovacuum_enabled = false);

CREATE UNLOGGED TABLE transacoes (
                            id SERIAL PRIMARY KEY,
                            cliente_id INTEGER NOT NULL,
                            valor INTEGER NOT NULL,
                            tipo CHAR(1) NOT NULL,
                            descricao VARCHAR(10) NOT NULL,
                            realizada_em TIMESTAMP NOT NULL DEFAULT NOW()
) WITH (autovacuum_enabled = false);

CREATE INDEX idx_cliente_id ON transacoes (cliente_id ASC);

INSERT INTO clientes (nome, limite, saldo)
VALUES
    ('o barato sai caro', 1000 * 100, 0),
    ('zan corp ltda', 800 * 100, 0),
    ('les cruders', 10000 * 100, 0),
    ('padaria joia de cocaia', 100000 * 100, 0),
    ('kid mais', 5000 * 100, 0);

CREATE OR REPLACE FUNCTION process_transaction(_cliente_id INT, _valor INT, _descricao TEXT, _tipo CHAR)
RETURNS INT
LANGUAGE plpgsql
AS $$
DECLARE
currentAmount INT;
    newAmount INT;
   	currentLimit INT;
BEGIN

SELECT limite, saldo INTO currentLimit, currentAmount
FROM clientes
WHERE id = _cliente_id FOR UPDATE;

if not found then
    return -10000000;
end if;

IF _tipo = 'd' THEN
        newAmount := currentAmount - _valor;
        IF newAmount < 0 AND ABS(newAmount) > currentLimit THEN
            return -100000001;
END IF;
    ELSIF _tipo = 'c' THEN
        newAmount := currentAmount + _valor;
END IF;

UPDATE clientes SET saldo = newAmount WHERE id = _cliente_id;

INSERT INTO transacoes (cliente_id, valor, descricao, tipo)
VALUES (_cliente_id, _valor, _descricao, _tipo);

RETURN newAmount;
END;
$$;