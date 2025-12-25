-- Seed default admin user
-- Email: admin@cold.com
-- Password: admin123

INSERT INTO users (name, email, password_hash, created_at, updated_at)
VALUES (
    'Administrator',
    'admin@cold.com',
    '$2a$12$eva1KcgmhwYUwDTOHQmyt.KZmPfAhvJuqbHIVLTzNbqSFISKqf8s6',
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
)
ON CONFLICT (email) DO NOTHING;
