-- test users for manual testing
INSERT INTO users (id, balance) VALUES
    (1, 100.00),
    (2, 50.00),
    (3, 0.00)
ON CONFLICT (id) DO NOTHING;

-- adjust sequence
SELECT setval('users_id_seq', (SELECT MAX(id) FROM users));
